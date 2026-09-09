package main

// Get your mind out of the gutter.

// Scenario 07 -- a consumer that starts at the head of the stream.
//
// The head is the newest message in the stream at the moment the consumer
// group is registered. A group that starts there reads only messages produced
// after it. The default, sqlstreams.Beginning(), reads every message ever stored.
//
// Moderation is added a year after videos.uploaded went live. It wants live
// uploads only rather than processing the entire archive, so the new consumer
// group starts at the head.
//
// Run first: 01

import (
	"context"
	"fmt"
	"os"

	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
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
	ctx, stop := sqlstreams.LifecycleContext(nil)
	defer stop()

	pool, err := sqlstreams.NewPostgresPool(ctx, "example_user", "example_password", "localhost", "example_db", nil)
	if err != nil {
		return err
	}
	defer pool.Close()

	client, err := sqlstreams.NewClient(ctx, pool, nil)
	if err != nil {
		return err
	}

	uploads := client.Stream[VideoUploadedV1]("videos.uploaded")
	moderation := uploads.Consumer("moderation")

	// skip the archive: only consume uploads produced after this registration
	consumer, err := moderation.Register(ctx, &sqlstreams.ConsumerConfig{
		Start: sqlstreams.Head(),
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
