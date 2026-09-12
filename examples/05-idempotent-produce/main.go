package main

// Scenario 05 -- idempotent produce with a caller-supplied key.
//
// Upload-complete webhooks arrive from a storage provider. The provider
// retries on any non-2xx, so the same upload arrives more than once and must
// be stored once on videos.uploaded.

import (
	"fmt"
	"os"

	sqlstreams "github.com/agentstax/sqlstreams/client"
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
	_, err = uploads.Register(ctx, nil)
	if err != nil {
		return err
	}

	producer, err := uploads.Producer().Register(ctx, nil)
	if err != nil {
		return err
	}

	// the storage provider delivers upl-123 twice
	// and is identified as a .Duplicate second time
	for range 2 {
		video := &VideoUploadedV1{
			VideoId:         "video-42",
			OwnerId:         "creator-7",
			UploadId:        "upl-123",
			DurationMinutes: 12,
			SourceStatus:    "ready",
		}
		produced, err := producer.Produce(ctx, video, &sqlstreams.ProduceOptions{
			IdempotencyKey: video.UploadId,
		})
		if err != nil {
			return err
		}
		fmt.Printf("id=%d duplicate=%v\n", produced.Id, produced.Duplicate)
	}
	return nil
}
