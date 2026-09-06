// Scenario 04 -- consume with retry and dead-lettering.
//
// FrameForge's transcoder from scenario 03 now handles real outcomes: a
// corrupt upload will never succeed (terminal), unavailable storage may
// recover (retry), and an embargoed video waits without counting as a failure
// (delay).
//
// Concepts held before domain code (10): the 7 from scenario 03, plus
// MessageOptions, RetryPolicy, and the ClientConfig.Retry vs
// ConsumerConfig.Message.Retry distinction.
//
// Traps hit:
//   - ClientConfig.Retry is the client's own Postgres retry;
//     ConsumerConfig.Message.Retry is message redelivery. Both are
//     *vulkan.RetryPolicy, so a curve meant for one type-checks in the
//     other; only the field comments separate them.
//   - Reading which attempt this is needs MetaFromContext -- the comma-ok
//     is always true inside a handler, yet every handler writes it.
//   - There is no dead-letter read verb on the consumer or admin; "what is
//     dead" is a psql query against exception_queue_<id>.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	vulkan "github.com/agentstax/vulkan/pkg/vulkan"
)

type VideoUploadedV1 struct {
	VideoId         string `json:"video_id"`
	OwnerId         string `json:"owner_id"`
	UploadId        string `json:"upload_id"`
	DurationMinutes int    `json:"duration_minutes"`
	SourceStatus    string `json:"source_status"` // "ready" | "corrupt" | "unavailable"
	ReleaseAtUnix   int64  `json:"release_at_unix"`
}

// increment on breaking changes
func (VideoUploadedV1) SchemaVersion() int { return 1 }

var (
	errSourceCorrupt      = errors.New("source video is corrupt")
	errStorageUnavailable = errors.New("source storage is unavailable")
)

func main() {
	if err := run(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := vulkan.LifecycleContext(nil)
	defer stop()

	pool, err := vulkan.NewPostgresPool(ctx, "example_user", "example_password", "localhost", "example_db", nil)
	if err != nil {
		return err
	}
	defer pool.Close()

	client, err := vulkan.NewClient(ctx, pool, nil)
	if err != nil {
		return err
	}
	transcoder, err := client.Topic[VideoUploadedV1]("videos.uploaded").Consumer("transcoder").Register(ctx, &vulkan.ConsumerConfig{
		Message: &vulkan.MessageOptions{
			Timeout: 10 * time.Second,
			Retry:   &vulkan.RetryPolicy{MaxRetries: 3, BaseDelay: 2 * time.Second},
		},
	})

	if err != nil {
		return err
	}

	return transcoder.Consume(ctx, func(ctx context.Context, video *VideoUploadedV1) error {
		meta, _ := vulkan.MetaFromContext(ctx)
		fmt.Printf("transcoding %s (message %d, attempt %d, delays %d)\n",
			video.VideoId, meta.Id, meta.Attempts+1, meta.Delays)

		switch video.SourceStatus {
		case "corrupt":
			// dead on this attempt; the cause lands in last_error
			return vulkan.Terminal(errSourceCorrupt)
		case "unavailable":
			// the first attempt retries with backoff; the next simulates recovery
			if meta.Attempts == 0 {
				return errStorageUnavailable
			}
		}
		releaseAt := time.Unix(video.ReleaseAtUnix, 0)
		if releaseAt.After(time.Now()) {
			// runs again after the release window
			return vulkan.Delay(time.Until(releaseAt))
		}
		return nil
	}, nil)
}
