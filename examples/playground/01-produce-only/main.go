// Scenario 01 -- produce-only service.
//
// FrameForge's upload API produces a message when a video finishes uploading.
// It never consumes anything.
//
// Concepts held before domain code (5): connection pool, Client, topic
// name, the Message type's SchemaVersion, RegisterProducer[T].
//
// Traps hit:
//   - Nothing here runs topic upkeep (partition create-ahead, retention).
//     A deployment of only this binary accumulates until someone runs
//     `vulkan manager run`. RegisterProducer now warns VK0063 naming the
//     unclaimed topic_janitor, so it is no longer silent -- but the warn
//     is the only thing that says so, and it is not an error.
//   - The pool is the one constructor before anything vulkan owns
//     [0633] [0636]. It buys the DATABASE_URL path and one pool per
//     application, and it costs a concept on every scenario's count.
package main

import (
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

	registered, err := client.Topic[VideoUploadedV1]("videos.uploaded").Register(ctx, nil)
	if err != nil {
		return err
	}

	uploads, err := client.Topic[VideoUploadedV1](registered.Name).Producer().Register(ctx, nil)
	if err != nil {
		return err
	}

	produced, err := uploads.Produce(ctx, &VideoUploadedV1{
		VideoId:         "video-42",
		OwnerId:         "creator-7",
		UploadId:        "upl-123",
		DurationMinutes: 12,
		SourceStatus:    "ready",
	}, nil)
	if err != nil {
		return err
	}
	fmt.Printf("produced id=%d\n", produced.Id)
	return nil
}
