package main

// compat drives the pinned release against system and stream tables prepared
// by the current build. It never creates either scope itself. Run through
// just compat-lab so go.work cannot substitute the working library.

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/allegedlyreliable/sqlstreams/client"
)

const messageCount = 5

// Message is the numbered JSON payload used to detect missing or corrupt data.
type Message int

func (Message) SchemaVersion() int { return 1 }

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	expect := flag.String("expect", "round-trip", `declared verdict: "round-trip" | "refused"`)
	name := flag.String("stream", "compat.lab", "stream already created and migrated by the current build")
	flag.Parse()
	if *expect != "round-trip" && *expect != "refused" {
		return fmt.Errorf("-expect must be round-trip or refused, got %q", *expect)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := sqlstreams.NewPostgresPool(ctx, "example_user", "example_password", "localhost", "example_db", nil)
	if err != nil {
		return err
	}
	defer pool.Close()
	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	if err != nil {
		return err
	}

	// Both reads must succeed before registration can run. Otherwise the old
	// build could create its own tables and never test the current schema.
	systemVersion, err := client.System().MigrationVersion(ctx)
	if err != nil {
		return fmt.Errorf("read current build's system schema: %w", err)
	}
	handle := client.Stream[Message](*name)
	streamVersion, err := handle.MigrationVersion(ctx)
	if err != nil {
		return fmt.Errorf("read current build's stream schema: %w", err)
	}
	if systemVersion < 1 || streamVersion < 1 {
		return fmt.Errorf("system/stream versions = %d/%d, want registered scopes", systemVersion, streamVersion)
	}
	registered, err := handle.Get(ctx)
	if err != nil {
		return err
	}
	if registered == nil {
		return fmt.Errorf("stream %q must be created by the current build first", *name)
	}
	fmt.Printf("stream %s id=%d, system schema=%d, stream schema=%d\n", *name, registered.Id, systemVersion, streamVersion)

	producer, err := handle.Producer().Register(ctx, nil)
	if *expect == "refused" {
		if !errors.Is(err, sqlstreams.ErrSchemaNewerThanBuild) {
			return fmt.Errorf("producer registration = %v, want ErrSchemaNewerThanBuild", err)
		}
		fmt.Println("COMPAT LAB PASSED (refused)")
		return nil
	}
	if err != nil {
		return err
	}
	for i := 1; i <= messageCount; i++ {
		message := Message(i)
		produced, err := producer.Produce(ctx, &message, nil)
		if err != nil {
			return fmt.Errorf("produce payload %d: %w", i, err)
		}
		fmt.Printf("produced payload=%d message_id=%d\n", i, produced.Id)
	}

	consumerHandle := handle.Consumer("compat.lab.group")
	consumer, err := consumerHandle.Register(ctx, nil)
	if err != nil {
		return err
	}
	group, err := consumerHandle.Get(ctx)
	if err != nil {
		return err
	}
	if group == nil {
		return errors.New("registered consumer group is missing")
	}
	fmt.Printf("consumer %s id=%d\n", group.Name, group.Id)

	consumeCtx, stop := context.WithCancel(ctx)
	defer stop()
	var seen [messageCount]atomic.Bool
	var consumed atomic.Int64
	err = consumer.Consume(consumeCtx, func(ctx context.Context, message *Message) error {
		sequence := int(*message)
		if sequence < 1 || sequence > messageCount {
			return fmt.Errorf("payload = %d, want 1 through %d", sequence, messageCount)
		}
		if !seen[sequence-1].Swap(true) && consumed.Add(1) == messageCount {
			stop()
		}
		return nil
	}, nil)
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	if consumed.Load() != messageCount {
		return fmt.Errorf("consumed %d distinct payloads, want %d", consumed.Load(), messageCount)
	}
	fmt.Printf("consumed all %d distinct payloads\n", messageCount)

	if err := handle.Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}); err != nil {
		return err
	}
	remaining, err := handle.Get(ctx)
	if err != nil {
		return err
	}
	if remaining != nil {
		return errors.New("destroyed stream still exists")
	}
	fmt.Println("COMPAT LAB PASSED (round-trip)")
	return nil
}
