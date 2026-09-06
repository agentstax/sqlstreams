// Scenario 09 -- keyed ordering: what a same-key consumer actually sees.
//
// FrameForge's uploaded -> scanned -> transcoded -> ready transitions for one
// video must apply in order and never overlap. The producer keys by video;
// the consumer runs concurrently across different videos.
//
// Concepts held before domain code (8): the produce set from scenario 01,
// plus MessageKey, ConsumerConfig.ConcurrencyOverride (ConcurrencyOrdered),
// the session's ConsumeOptions.MessageConcurrency, and the "ordered =
// every same-key message in id order, one at a time, through failures"
// semantics.
//
// Traps hit:
//   - A message key alone orders nothing: MessageConcurrency > 1 delivers
//     two same-key messages at once unless the group declares
//     ConcurrencyOverride. The per-message MessageOptions.Concurrency form
//     also exists, and a second producer that sets the key without it
//     runs in parallel with the first -- so the group form leads here.
package main

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
	registered, err := client.Topic[VideoStateChangedV1]("videos.state-changed").Register(ctx, nil)
	if err != nil {
		return err
	}
	states, err := client.Topic[VideoStateChangedV1](registered.Name).Producer().Register(ctx, nil)
	if err != nil {
		return err
	}
	catalog, err := client.Topic[VideoStateChangedV1](registered.Name).Consumer("video-catalog").Register(ctx, &vulkan.ConsumerConfig{ConcurrencyOverride: vulkan.ConcurrencyOrdered})
	if err != nil {
		return err
	}

	for _, videoId := range []string{"video-42", "video-43"} {
		for _, state := range []string{"uploaded", "scanned", "transcoded", "ready"} {
			if _, err := states.Produce(ctx, &VideoStateChangedV1{VideoId: videoId, State: state}, &vulkan.ProduceOptions{MessageKey: videoId}); err != nil {
				return err
			}
		}
	}

	return catalog.Consume(ctx, applyStateChange, &vulkan.ConsumeOptions{MessageConcurrency: 8})
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
