package main

// Scenario 08 -- keyed ordering: what a same-key consumer actually sees.
//
// The uploaded -> scanned -> transcoded -> ready transitions for one video
// must apply in order and never overlap. The producer keys by video; the
// consumer runs concurrently across different videos.

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/allegedlyreliable/sqlstreams/client"
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

	states := client.Stream[VideoStateChangedV1]("videos.state-changed")
	_, err = states.Register(ctx, nil)
	if err != nil {
		return err
	}

	producer, err := states.Producer().Register(ctx, nil)
	if err != nil {
		return err
	}

	// sqlstreams.ConcurrencyOrdered            -> one video's states run one at a time, in produce order
	//                                         a failed state holds the later ones until its retry succeeds
	//                                         or it exhausts its retries and is marked dead
	// sqlstreams.ConcurrencyParallel (default) -> same-key states overlap, so video-42 can go "ready"
	//                                         before its "scanned" retry lands
	catalog := states.Consumer("video-catalog")
	consumer, err := catalog.Register(ctx, &sqlstreams.ConsumerConfig{ConcurrencyOverride: sqlstreams.ConcurrencyOrdered})
	if err != nil {
		return err
	}

	// the message key is what ordering is scoped by: same key, same line;
	// a message without a key has nothing to order against and runs freely
	for _, videoId := range []string{"video-42", "video-43"} {
		for _, state := range []string{"uploaded", "scanned", "transcoded", "ready"} {
			if _, err := producer.Produce(ctx, &VideoStateChangedV1{VideoId: videoId, State: state}, &sqlstreams.ProduceOptions{MessageKey: videoId}); err != nil {
				return err
			}
		}
	}

	// concurrency 8 still applies across videos: video-43 runs alongside
	// video-42, ordering only serializes within one key
	return consumer.Consume(ctx, applyStateChange, &sqlstreams.ConsumeOptions{MessageConcurrency: 8})
}

// applyStateChange fails video-42's scan once; its later states wait for the retry.
func applyStateChange(ctx context.Context, change *VideoStateChangedV1) error {
	meta, _ := sqlstreams.MetaFromContext(ctx)
	if change.VideoId == "video-42" && change.State == "scanned" && meta.Attempts == 0 {
		return errors.New("scanner result is not committed")
	}
	fmt.Printf("%s -> %s (message %d, attempt %d)\n", change.VideoId, change.State, meta.Id, meta.Attempts+1)
	return nil
}
