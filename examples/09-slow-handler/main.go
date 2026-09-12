package main

// Scenario 09 -- a handler that runs longer than its lease.
//
// The transcoder from scenarios 02 and 03 usually finishes quickly, but
// feature-length videos can take an hour. The handler cannot know its actual
// runtime up front, so the timeout is resolved per message.
//
// How timeout resolves, highest first:
//	consumer clamp (MessageMin/MessageMax) > produced message > producer defaults > consumer defaults > system defaults
//
// Run first: 01

import (
	"context"
	"fmt"
	"os"
	"time"

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
	producer, err := uploads.Producer().Register(ctx, nil)
	if err != nil {
		return err
	}

	// ConsumerConfig.Message    -> the timeout a message gets when it asks for nothing (most videos)
	// ConsumerConfig.MessageMax -> the most a message may ask for; a request above it is lowered to it
	transcoder := uploads.Consumer("transcoder")
	consumer, err := transcoder.Register(ctx, &sqlstreams.ConsumerConfig{
		Message:    &sqlstreams.MessageOptions{Timeout: 2 * time.Minute},
		MessageMax: &sqlstreams.MessageOptions{Timeout: time.Hour},
	})
	if err != nil {
		return err
	}

	// ProduceOptions.Message    -> what this one message asks for; the producer knows the upload is feature-length
	if _, err := producer.Produce(ctx, &VideoUploadedV1{
		VideoId:         "video-99",
		OwnerId:         "creator-7",
		UploadId:        "upl-999",
		DurationMinutes: 95,
		SourceStatus:    "ready",
	}, &sqlstreams.ProduceOptions{Message: &sqlstreams.MessageOptions{Timeout: time.Hour}}); err != nil {
		return err
	}

	return consumer.Consume(ctx, transcodeVideo, nil)
}

func transcodeVideo(ctx context.Context, video *VideoUploadedV1) error {
	// past the timeout ctx is cancelled, not the goroutine: a handler that
	// ignores ctx.Done() keeps running while the message is redelivered
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
