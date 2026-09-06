package main

// Scenario 09 -- keyed ordering: what a same-key consumer actually sees.
//
// FrameForge's uploaded -> scanned -> transcoded -> ready transitions for one
// video must apply in order and never overlap. The producer keys by video;
// the consumer runs concurrently across different videos.

import (
	"context"
	"errors"
	"fmt"
	"os"

	vulkan "github.com/agentstax/vulkan/pkg/vulkan"
)

type VideoStateChangedV1 struct {
	VideoId string `json:"video_id"`
	State   string `json:"state"`
}

// increment on breaking changes
func (VideoStateChangedV1) SchemaVersion() int { return 1 }

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

	states := client.Topic[VideoStateChangedV1]("videos.state-changed")
	_, err = states.Register(ctx, nil)
	if err != nil {
		return err
	}

	producer, err := states.Producer().Register(ctx, nil)
	if err != nil {
		return err
	}

	catalog := states.Consumer("video-catalog")
	consumer, err := catalog.Register(ctx, &vulkan.ConsumerConfig{ConcurrencyOverride: vulkan.ConcurrencyOrdered})
	if err != nil {
		return err
	}

	for _, videoId := range []string{"video-42", "video-43"} {
		for _, state := range []string{"uploaded", "scanned", "transcoded", "ready"} {
			if _, err := producer.Produce(ctx, &VideoStateChangedV1{VideoId: videoId, State: state}, &vulkan.ProduceOptions{MessageKey: videoId}); err != nil {
				return err
			}
		}
	}

	return consumer.Consume(ctx, applyStateChange, &vulkan.ConsumeOptions{MessageConcurrency: 8})
}

// applyStateChange fails video-42's scan once; its later states wait for the retry.
func applyStateChange(ctx context.Context, change *VideoStateChangedV1) error {
	meta, _ := vulkan.MetaFromContext(ctx)
	if change.VideoId == "video-42" && change.State == "scanned" && meta.Attempts == 0 {
		return errors.New("scanner result is not committed")
	}
	fmt.Printf("%s -> %s (message %d, attempt %d)\n", change.VideoId, change.State, meta.Id, meta.Attempts+1)
	return nil
}
