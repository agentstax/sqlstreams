package main

// Scenario 05 -- a compacted topic used as a key/value store.
//
// FrameForge keeps one current processing document per video id. Read the
// current value, write a new one, and increment its attempt count safely
// under concurrent writers (read-modify-write).

import (
	"context"
	"fmt"
	"os"

	vulkan "github.com/agentstax/vulkan/pkg/vulkan"
)

type VideoProcessingStateV1 struct {
	VideoId  string `json:"video_id"`
	Stage    string `json:"stage"`
	Attempts int    `json:"attempts"`
}

// increment on breaking changes
func (VideoProcessingStateV1) SchemaVersion() int { return 1 }

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

	states := client.Topic[VideoProcessingStateV1]("videos.processing-state")
	_, err = states.Register(ctx, nil)
	if err != nil {
		return err
	}

	producer, err := states.Producer().Register(ctx, nil)
	if err != nil {
		return err
	}

	video := states.Key("video-42")

	// Put
	_, err = producer.Produce(ctx, &VideoProcessingStateV1{VideoId: "video-42", Stage: "transcoding", Attempts: 1},
		&vulkan.ProduceOptions{MessageKey: "video-42", Compaction: &vulkan.CompactionOptions{Enable: true}})
	if err != nil {
		return err
	}

	// Get (outside a transaction) -- the topic handle's read
	current, err := video.CompactionHead(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("current: id=%d stage=%s attempts=%d\n", current.Id, current.Message.Stage, current.Message.Attempts)

	// Update (compare-and-set): lock the head, write the next version
	if err := client.InTransaction(ctx, func(ctx context.Context, tx vulkan.Tx) error {
		head, err := video.LockCompactionHead(ctx, tx)
		if err != nil {
			return err
		}
		next := VideoProcessingStateV1{VideoId: "video-42", Stage: "transcoding"}
		if head != nil {
			next = *head.Message
		}
		next.Attempts++
		_, err = producer.ProduceInTx(ctx, tx, &next, &vulkan.ProduceOptions{MessageKey: "video-42", Compaction: &vulkan.CompactionOptions{Enable: true}})
		return err
	}); err != nil {
		return err
	}

	// History
	versions, err := video.Messages(ctx, 10)
	if err != nil {
		return err
	}
	for _, version := range versions {
		fmt.Printf("version id=%d attempts=%d\n", version.Id, version.Message.Attempts)
	}
	return nil
}
