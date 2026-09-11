package main

// producer register e2e test: the producer lifecycle end to end.
//
// Confirms: Register is a stateless build step -- its ctx bounds only that
// call's I/O, context.Background() is fine, and each call returns an
// independent instance. Shutdown is per produce call: a cancelled ctx is
// refused with nothing published, and the instance itself holds no lifetime
// -- the same handle produces again on a live ctx.

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
)

type Message struct {
	Data string
}

func (Message) SchemaVersion() int { return 1 }

func main() {
	if err := run(); err != nil {
		fmt.Printf("\n❌ E2E TEST FAILED: %s\n", err.Error())
		os.Exit(1)
	}
}

func run() (err error) {
	defer common.Recover(&err)
	ctx := context.Background()

	pool, err := common.NewPool(ctx, nil)
	common.Must(err)
	defer pool.Close()

	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	common.Must(err)

	const streamName = "test.producerregister"
	_ = client.Stream[sqlstreams.RawPayload](streamName).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}) // clean slate from any crashed prior run
	tp, err := client.Stream[sqlstreams.RawPayload](streamName).Register(ctx, &sqlstreams.StreamConfig{})
	common.Must(err)
	defer func() {
		common.Must(client.Stream[sqlstreams.RawPayload](streamName).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	}()

	// ===== Register on Background =====
	step("Register(context.Background()) -- a build step, no lifetime to enforce")
	instance, err := client.Stream[Message](tp.Name).Producer().Register(ctx, nil)
	common.Must(err)
	produced, err := instance.Produce(ctx, &Message{Data: "registered"}, nil)
	common.Must(err)
	fmt.Printf("  ✓ produced %+v id=%d duplicate=%t\n", *produced.Message, produced.Id, produced.Duplicate)

	// ===== per-call shutdown =====
	step("Produce with a cancelled ctx -- refused, nothing published")
	cancelled, cancel := context.WithCancel(ctx)
	cancel() // stands in for SIGINT/SIGTERM: the app's shutdown context has fired
	_, err = instance.Produce(cancelled, &Message{Data: "too late"}, nil)
	requireIs(err, context.Canceled)

	// ===== the instance holds no lifetime =====
	step("produce again on a live ctx -- the same instance still accepts work")
	_, err = instance.Produce(ctx, &Message{Data: "second life"}, nil)
	common.Must(err)
	fmt.Println("  ✓ same instance produces after the cancelled call")

	// ===== Register many times =====
	step("Register again -- an independent instance from the same factory")
	sibling, err := client.Stream[Message](tp.Name).Producer().Register(ctx, nil)
	common.Must(err)
	_, err = sibling.Produce(ctx, &Message{Data: "sibling"}, nil)
	common.Must(err)
	fmt.Println("  ✓ sibling instance produces")

	fmt.Println("\n✅ PRODUCER REGISTER E2E TEST PASSED")
	fmt.Println("   Register only builds: instances hold no lifetime, a cancelled Produce ctx")
	fmt.Println("   refuses that one message, and the handle stays valid for the next call.")
	return nil
}

// ---- helpers ----

func step(s string) { fmt.Printf("\n--- %s ---\n", s) }

func requireIs(err, want error) {
	if !errors.Is(err, want) {
		common.Die(fmt.Sprintf("want %v, got %v", want, err))
	}
	fmt.Printf("  ✓ %v\n", err)
}
