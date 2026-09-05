// Scenario 05 -- a compacted topic used as a key/value store.
//
// Device configuration: one current document per device id. Read the
// current value, write a new one, and increment a counter safely under
// concurrent writers (read-modify-write).
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

type DeviceConfig struct {
	DeviceId string `json:"device_id"`
	Interval int    `json:"interval_seconds"`
	Restarts int    `json:"restarts"`
}

// increment on breaking changes
func (DeviceConfig) SchemaVersion() int { return 1 }

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
	devices := client.Topic[DeviceConfig]("devices.config")
	_, err = devices.Register(ctx, nil)
	if err != nil {
		return err
	}

	configs, err := devices.Producer().Register(ctx, nil)
	if err != nil {
		return err
	}

	device := devices.Key("dev-7")
	compaction, err := vulkan.NewCompactionOptions(0)
	if err != nil {
		return err
	}

	// Put
	_, err = configs.Produce(ctx, &DeviceConfig{DeviceId: "dev-7", Interval: 30},
		&vulkan.ProduceOptions{MessageKey: "dev-7", Compaction: compaction})
	if err != nil {
		return err
	}

	// Get (outside a transaction) -- the topic handle's read
	current, err := device.CompactionHead(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("current: id=%d interval=%d restarts=%d\n", current.Id, current.Message.Interval, current.Message.Restarts)

	// Update (compare-and-set): lock the head, write the next version
	if err := client.InTransaction(ctx, func(ctx context.Context, tx vulkan.Tx) error {
		head, err := device.LockCompactionHead(ctx, tx)
		if err != nil {
			return err
		}
		next := DeviceConfig{DeviceId: "dev-7"}
		if head != nil {
			next = *head.Message
		}
		next.Restarts++
		_, err = configs.ProduceInTx(ctx, tx, &next, &vulkan.ProduceOptions{MessageKey: "dev-7", Compaction: compaction})
		return err
	}); err != nil {
		return err
	}

	// History
	versions, err := device.Messages(ctx, 10)
	if err != nil {
		return err
	}
	for _, version := range versions {
		fmt.Printf("version id=%d restarts=%d\n", version.Id, version.Message.Restarts)
	}
	return nil
}
