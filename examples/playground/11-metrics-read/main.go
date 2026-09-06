package main

// Scenario 11 -- reading what the system measures about itself.
//
// FrameForge's transcoder consumes the uploads introduced in scenario 01 while
// the manager's metrics collector measures the system. A loop prints the
// transcoder's live backlog beside its last collected value -- the pull side
// an operations dashboard would use.

import (
	"context"
	"fmt"
	"os"
	"time"

	vulkan "github.com/agentstax/vulkan/pkg/vulkan"
	"golang.org/x/sync/errgroup"
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
	_, err = uploads.Register(ctx, nil)
	if err != nil {
		return err
	}

	producer, err := uploads.Producer().Register(ctx, nil)
	if err != nil {
		return err
	}
	if err := produceVideos(ctx, producer, 5); err != nil {
		return err
	}

	transcoder := uploads.Consumer("transcoder")
	consumer, err := transcoder.Register(ctx, nil)
	if err != nil {
		return err
	}

	routines, routinesCtx := errgroup.WithContext(ctx)
	routines.Go(func() error { return consumer.Consume(routinesCtx, transcodeVideo, nil) })
	routines.Go(func() error { return printBacklog(routinesCtx, transcoder.Metrics()) })
	return routines.Wait()
}

func produceVideos(ctx context.Context, producer *vulkan.ProducerInstance[VideoUploadedV1], count int) error {
	for i := range count {
		video := &VideoUploadedV1{
			VideoId:         fmt.Sprintf("video-%d", i+42),
			OwnerId:         "creator-7",
			UploadId:        fmt.Sprintf("upl-%d", i+123),
			DurationMinutes: 12,
			SourceStatus:    "ready",
		}
		if _, err := producer.Produce(ctx, video, nil); err != nil {
			return err
		}
	}
	return nil
}

// transcodeVideo is slow so the backlog drains over ~50s, longer than the
// collector's 30s poll, and the two backlog numbers printBacklog reads diverge.
func transcodeVideo(ctx context.Context, video *VideoUploadedV1) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Second):
		return nil
	}
}

// printBacklog prints the group's live backlog beside its last collected one.
func printBacklog(ctx context.Context, transcoderMetrics *vulkan.ConsumerMetricsHandle) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}

		snapshot, err := transcoderMetrics.Snapshot(ctx)
		if err != nil {
			return err
		}
		live := snapshot.Cursor.Backlog

		collected, err := transcoderMetrics.CursorBacklog().Latest(ctx)
		if err != nil {
			return err
		}
		if collected == nil {
			fmt.Printf("live backlog %d, nothing collected yet\n", live)
			continue
		}
		fmt.Printf("live backlog %d, collected backlog %g as of %s ago\n",
			live, collected.Value, time.Since(collected.At).Round(time.Second))
	}
}
