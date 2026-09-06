// Scenario 10 -- a handler that runs longer than its lease.
//
// FrameForge's transcoder from scenarios 03 and 04 usually finishes quickly,
// but feature-length videos can take an hour. The handler cannot know its
// actual runtime up front.
//
// Concepts held before domain code (12): the 7 from scenario 03, plus
// MessageOptions.Timeout, MessageMax, the producer-side per-message
// Timeout request, the lease = Timeout + grace + margins formula, and the
// ctx.Done() contract inside the handler.
//
// Traps hit:
//   - There is no way to extend a lease from inside the handler (SQS
//     ChangeMessageVisibility, JetStream InProgress). The only knob is a
//     ceiling chosen before the work starts: set it to the worst case and
//     a crashed consumer instance's message waits an hour to be reclaimed;
//     set it to the common case and long videos time out and retry forever.
//   - The timeout is decided by three parties -- the message's request,
//     the consumer's default, the consumer's MessageMax clamp -- and a
//     message asking for more than MessageMax is clamped silently (a Warn
//     log, not an error).
//   - A handler that ignores ctx.Done() past the timeout is abandoned, not
//     killed: it keeps running while the message is redelivered elsewhere.
package main

import (
	"context"
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
	SourceStatus    string `json:"source_status"`
	ReleaseAtUnix   int64  `json:"release_at_unix"`
}

// increment on breaking changes
func (VideoUploadedV1) SchemaVersion() int { return 1 }

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
		Message:    &vulkan.MessageOptions{Timeout: 2 * time.Minute},
		MessageMax: &vulkan.MessageOptions{Timeout: time.Hour},
	})

	if err != nil {
		return err
	}

	return transcoder.Consume(ctx, func(ctx context.Context, video *VideoUploadedV1) error {
		for minute := range video.DurationMinutes {
			// TODO - wanted: "still working, extend my lease" -- nothing to call.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Minute):
				fmt.Printf("%s: minute %d\n", video.VideoId, minute+1)
			}
		}
		return nil
	}, nil)
}
