package main

// ListConsumers e2e test: proves the consumer-group list read and the Stream / Consumer
// handles against a live database -- a stream's groups list in name order,
// Consumer.Get returns the row, absence is (nil, nil) on Get and
// ErrStreamNotFound on every other verb, and Stream.Destroy drops the family.

import (
	"context"
	"errors"
	"fmt"
	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	"github.com/agentstax/sqlstreams/pkg/datastore"
	"os"
	"time"

	"github.com/agentstax/sqlstreams/pkg/consume"
	consumecontroller "github.com/agentstax/sqlstreams/pkg/consume/controller"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/agentstax/sqlstreams/pkg/stream"
)

func main() {
	if err := run(); err != nil {
		fmt.Printf("\n❌ E2E TEST FAILED: %s\n", err.Error())
		os.Exit(1)
	}
}

func run() (err error) {
	defer common.Recover(&err)
	ctx := context.Background()
	run := time.Now().UnixNano()

	pool, err := common.NewPool(ctx, nil)
	common.Must(err)
	defer pool.Close()

	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	common.Must(err)
	ds, err := datastore.NewPostgresDatastore(ctx, pool, nil)
	common.Must(err)

	name := fmt.Sprintf("listgroups.orders.%d", run)
	registered, err := client.Stream[sqlstreams.RawPayload](name).Register(ctx, nil)
	common.Must(err)

	groupController, err := consumecontroller.NewConsumeController(ds, ds.Logger)
	common.Must(err)

	step("seed two groups on the stream")
	for _, groupName := range []string{"beta", "alpha"} {
		_, err := groupController.RegisterGroup(ctx, registered.Id, groupName, consume.Beginning())
		common.Must(err)
	}

	step("StreamHandle.Consumers returns both, ordered by name")
	orders := client.Stream[sqlstreams.RawPayload](name)
	groups, err := orders.Consumers(ctx)
	common.Must(err)
	if len(groups) != 2 {
		common.Die(fmt.Sprintf("expected 2 groups, got %d", len(groups)))
	}
	assertString("first group", groups[0].Name, "alpha")
	assertString("second group", groups[1].Name, "beta")
	assertInt64("group stream id", groups[0].StreamId, registered.Id)

	step("Consumer.Get returns the row")
	alpha, err := orders.Consumer("alpha").Get(ctx)
	common.Must(err)
	if alpha == nil {
		common.Die("expected the alpha row, got nil")
	}
	assertInt64("alpha id", alpha.Id, groups[0].Id)

	step("absence is (nil, nil) on Get only")
	ghost, err := orders.Consumer("ghost").Get(ctx)
	common.Must(err)
	if ghost != nil {
		common.Die(fmt.Sprintf("expected (nil, nil) for an unregistered group, got %+v", ghost))
	}
	ghostStream := client.Stream[sqlstreams.RawPayload](fmt.Sprintf("listgroups.ghost.%d", run))
	row, err := ghostStream.Get(ctx)
	common.Must(err)
	if row != nil {
		common.Die(fmt.Sprintf("expected (nil, nil) for an unregistered stream, got %+v", row))
	}
	_, err = ghostStream.Consumers(ctx)
	if !errors.Is(err, stream.ErrStreamNotFound) {
		common.Die(fmt.Sprintf("Consumers on an unregistered stream: expected ErrStreamNotFound, got %v", err))
	}
	fmt.Printf("  ✓ Consumers on an unregistered stream -> %v\n", err)

	step("cleanup: Stream.Destroy drops the family")
	common.Must(orders.Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	gone, err := orders.Get(ctx)
	common.Must(err)
	if gone != nil {
		common.Die("expected the stream row gone after Destroy")
	}

	fmt.Println("\n✅ LIST GROUPS E2E TEST PASSED")
	return nil
}

func assertString(label string, got string, want string) {
	if got != want {
		common.Die(fmt.Sprintf("%s: got %q, want %q", label, got, want))
	}
	fmt.Printf("  ✓ %s (%q)\n", label, got)
}

func assertInt64(label string, got int64, want int64) {
	if got != want {
		common.Die(fmt.Sprintf("%s: got %d, want %d", label, got, want))
	}
	fmt.Printf("  ✓ %s (%d)\n", label, got)
}

func step(s string) { fmt.Printf("\n--- %s ---\n", s) }
