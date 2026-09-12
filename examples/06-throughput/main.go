package main

// Scenario 06 -- tuning a stream for throughput.
//
// One finished upload asks for a thumbnail every few seconds of video, so a
// single upload becomes hundreds of small unkeyed messages. Concurrent
// producers are batched into shared transactions for free; the consumer
// claims, prefetches, and processes many at once.

import (
	"context"
	"fmt"
	"os"
	"time"

	sqlstreams "github.com/agentstax/sqlstreams/client"
	"golang.org/x/sync/errgroup"
)

type ThumbnailRequestedV1 struct {
	VideoId       string `json:"video_id"`
	OffsetSeconds int    `json:"offset_seconds"`
}

// increment on breaking changes
func (ThumbnailRequestedV1) SchemaVersion() int { return 1 }

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

	thumbnails := client.Stream[ThumbnailRequestedV1]("thumbnails.requested")
	_, err = thumbnails.Register(ctx, nil)
	if err != nil {
		return err
	}

	producer, err := thumbnails.Producer().Register(ctx, nil)
	if err != nil {
		return err
	}

	renderer := thumbnails.Consumer("thumbnail-renderer")
	consumer, err := renderer.Register(ctx, nil)
	if err != nil {
		return err
	}

	// Produce -> concurrent calls share a transaction without asking: under load they
	//            commit in batches (ProducerConfig.Batch sets the size)
	routines, routinesCtx := errgroup.WithContext(ctx)
	for offset := range 1000 {
		routines.Go(func() error {
			_, err := producer.Produce(routinesCtx, &ThumbnailRequestedV1{VideoId: "video-42", OffsetSeconds: offset * 5}, nil)
			return err
		})
	}

	// BatchLimit         -> messages claimed per poll (default 1)
	// QueueSize          -> messages held ready ahead of the handlers, so the next claim
	//                       runs while this one is still being processed (default BatchLimit)
	// MessageConcurrency -> handlers running at once; unkeyed messages have no order to keep (default 1)
	// ClaimPollRate      -> how long an instance that found nothing waits before claiming again (default 5s)
	routines.Go(func() error {
		return consumer.Consume(routinesCtx, renderThumbnail, &sqlstreams.ConsumeOptions{
			BatchLimit:         100,
			QueueSize:          200,
			MessageConcurrency: 16,
			ClaimPollRate:      time.Second,
		})
	})
	return routines.Wait()
}

func renderThumbnail(ctx context.Context, request *ThumbnailRequestedV1) error {
	fmt.Printf("rendered %s at %ds\n", request.VideoId, request.OffsetSeconds)
	return nil
}
