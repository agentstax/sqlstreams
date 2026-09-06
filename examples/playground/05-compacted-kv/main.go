// Scenario 05 -- a compacted topic used as a key/value store.
//
// FrameForge keeps one current processing document per video id. Read the
// current value, write a new one, and increment its attempt count safely
// under concurrent writers (read-modify-write).
//
// Concepts held before domain code (15): the 5 from scenario 01, plus
// MessageKey, CompactionOptions (+NewCompactionOptions), Rank,
// InTransaction, LockCompactionHead, ProduceInTx, Message, and the Topic and
// Key handles.
//
// Traps hit:
//   - "Compacted" is a per-message option, not a topic property: every
//     produce must pass Compaction or the message silently is not one
//     version of the key -- it is its own message forever.
//   - The Key handle owns both ordinary and transactional head reads; the
//     latter locks the head in the caller's transaction for ProduceInTx.
//   - CAS exists only as a pattern: InTransaction + LockCompactionHead
//     (FOR UPDATE) + ProduceInTx. Nothing named Update/Put says so.
//   - Rank is a commitment, not a hint; the zero value (arrival order) is
//     what most users want and NewCompactionOptions(0) reads like "no rank".
package main

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
	processingStates := client.Topic[VideoProcessingStateV1]("videos.processing-state")
	_, err = processingStates.Register(ctx, nil)
	if err != nil {
		return err
	}

	states, err := processingStates.Producer().Register(ctx, nil)
	if err != nil {
		return err
	}

	video := processingStates.Key("video-42")
	compaction, err := vulkan.NewCompactionOptions(0)
	if err != nil {
		return err
	}

	// Put
	_, err = states.Produce(ctx, &VideoProcessingStateV1{VideoId: "video-42", Stage: "transcoding", Attempts: 1},
		&vulkan.ProduceOptions{MessageKey: "video-42", Compaction: compaction})
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
		_, err = states.ProduceInTx(ctx, tx, &next, &vulkan.ProduceOptions{MessageKey: "video-42", Compaction: compaction})
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
