package main

// Scenario 02 -- consume-only service.
//
// A transcoder only handles VideoUploaded. It owns no stream and needs no
// admin verbs -- consumer registration resolves the stream by name.
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
	transcoder := uploads.Consumer("transcoder")
	consumer, err := transcoder.Register(ctx, nil)
	if err != nil {
		return err
	}

	// blocking
	return consumer.Consume(ctx, transcodeVideo, nil)
}

func transcodeVideo(ctx context.Context, video *VideoUploadedV1) error {
	fmt.Printf("transcoding %s (%d minutes)\n", video.VideoId, video.DurationMinutes)
	return nil
}
