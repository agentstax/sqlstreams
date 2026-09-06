// Scenario 08 -- idempotent produce with a caller-supplied key.
//
// FrameForge receives upload-complete webhooks from its storage provider. The
// provider retries on any non-2xx, so the same upload arrives more than once
// and must be stored once on videos.uploaded.
//
// Concepts held before domain code (7): the produce set from scenario 01,
// plus IdempotencyKey as an opaque string and ProduceResult.Duplicate.
//
// Traps hit:
//   - Duplicate is a field on a success result, not an error; a caller
//     that only checks err treats the duplicate as a fresh produce.
//   - A caller-supplied key opts the call out of batching (documented in
//     the field comment only).
//   - The idempotency window is IdempotencyKeyTTL on the TOPIC (24h) --
//     an upstream retrying after a day double-stores, silently.
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
	ctx := context.Background()

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

	// the storage provider delivers upl-123 twice
	for range 2 {
		video := &VideoUploadedV1{
			VideoId:         "video-42",
			OwnerId:         "creator-7",
			UploadId:        "upl-123",
			DurationMinutes: 12,
			SourceStatus:    "ready",
		}
		produced, err := uploads.Produce(ctx, video, &vulkan.ProduceOptions{
			IdempotencyKey: video.UploadId,
		})
		if err != nil {
			return err
		}
		fmt.Printf("id=%d duplicate=%v\n", produced.Id, produced.Duplicate)
	}
	return nil
}
