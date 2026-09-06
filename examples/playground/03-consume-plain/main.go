// Scenario 03 -- consume, plain.
//
// FrameForge's transcoder only handles VideoUploaded. It owns no topic and
// needs no admin verbs -- RegisterConsumer resolves the topic by name itself.
//
// Concepts held before domain code (7): connection pool, LifecycleContext,
// the Message type's SchemaVersion, Client, RegisterConsumer[T], consumer
// group name, the nil group config (whole topic, defaults).
//
// Traps hit:
//   - context.Background() into Consume fails with VK0002; the fix is a
//     Vulkan-specific ctx constructor the user must discover.
//   - Two `ctx` shapes in one file: the lifecycle ctx for Consume, and the
//     per-message ctx handed to the handler.
package main

import (
	"context"
	"fmt"
	"os"

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

	transcoder, err := client.Topic[VideoUploadedV1]("videos.uploaded").Consumer("transcoder").Register(ctx, nil)
	if err != nil {
		return err
	}

	return transcoder.Consume(ctx, func(ctx context.Context, video *VideoUploadedV1) error {
		fmt.Printf("transcoding %s (%d minutes)\n", video.VideoId, video.DurationMinutes)
		return nil
	}, nil)
}
