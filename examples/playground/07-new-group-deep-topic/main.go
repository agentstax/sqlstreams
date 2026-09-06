package main

// Scenario 07 -- a new consumer group on a topic with deep history.
//
// FrameForge adds moderation a year after videos.uploaded went live. It wants
// live uploads only rather than processing the entire archive.

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

	uploads := client.Topic[VideoUploadedV1]("videos.uploaded")
	moderation := uploads.Consumer("moderation")
	consumer, err := moderation.Register(ctx, &vulkan.ConsumerConfig{
		Start: vulkan.Head(),
	})
	if err != nil {
		return err
	}

	return consumer.Consume(ctx, moderateVideo, nil)
}

func moderateVideo(ctx context.Context, video *VideoUploadedV1) error {
	fmt.Printf("moderating %s for %s\n", video.VideoId, video.OwnerId)
	return nil
}
