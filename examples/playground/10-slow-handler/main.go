package main

// Scenario 10 -- a handler that runs longer than its lease.
//
// FrameForge's transcoder from scenarios 03 and 04 usually finishes quickly,
// but feature-length videos can take an hour. The handler cannot know its
// actual runtime up front.

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

	uploads := client.Topic[VideoUploadedV1]("videos.uploaded")
	transcoder := uploads.Consumer("transcoder")
	consumer, err := transcoder.Register(ctx, &vulkan.ConsumerConfig{
		Message:    &vulkan.MessageOptions{Timeout: 2 * time.Minute},
		MessageMax: &vulkan.MessageOptions{Timeout: time.Hour},
	})
	if err != nil {
		return err
	}

	return consumer.Consume(ctx, transcodeVideo, nil)
}

func transcodeVideo(ctx context.Context, video *VideoUploadedV1) error {
	for minute := range video.DurationMinutes {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Minute):
			fmt.Printf("%s: minute %d\n", video.VideoId, minute+1)
		}
	}
	return nil
}
