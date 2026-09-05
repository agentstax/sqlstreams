// Scenario 11 -- reading what the system measures about itself.
//
// A service that consumes orders while the manager's metrics collector
// measures the fleet, plus a loop printing the group's live backlog beside
// its last collected value -- the pull side an ops dashboard would use.
//
// Concepts held before domain code (11): the 7 from scenario 03, plus a
// ConsumerMetricsHandle, its Snapshot, its typed CursorBacklog selector, and
// the retained Latest measurement. The collector runs because Consume runs
// the manager -- errgroup is here for the print loop, not for it.
//
// Traps hit:
//   - A retained measurement exists only after the manager's metrics collector
//     ticks (30s default poll): Latest initially returns nil, and nothing on
//     ClientConfig or RegisterSystemConfig sets the collector's rate -- it
//     is worker metadata the client surface never reaches.
//   - Latest is the newest collected value, not live state. Measurement.At is
//     the observation time; Snapshot asks the source tables what is true now.
//   - The Group metrics handle supplies topic and group attributes. A typed
//     selector avoids copying the metric's wire name or assembling its key.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	vulkan "github.com/agentstax/vulkan/pkg/vulkan"
	"golang.org/x/sync/errgroup"
)

type OrderPlaced struct {
	OrderId string `json:"order_id"`
}

// increment on breaking changes
func (OrderPlaced) SchemaVersion() int { return 1 }

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
	registered, err := client.Topic[OrderPlaced]("orders.placed").Register(ctx, nil)
	if err != nil {
		return err
	}

	orders, err := client.Topic[OrderPlaced](registered.Name).Producer().Register(ctx, nil)
	if err != nil {
		return err
	}
	if err := produceOrders(ctx, orders, 5); err != nil {
		return err
	}

	ledger := client.Topic[OrderPlaced](registered.Name).Consumer("ledger")
	session, err := ledger.Register(ctx, nil)
	if err != nil {
		return err
	}

	routines, routinesCtx := errgroup.WithContext(ctx)
	routines.Go(func() error { return session.Consume(routinesCtx, recordOrder, nil) })
	routines.Go(func() error { return printBacklog(routinesCtx, ledger.Metrics()) })
	return routines.Wait()
}

func produceOrders(ctx context.Context, orders *vulkan.ProducerInstance[OrderPlaced], count int) error {
	for i := range count {
		if _, err := orders.Produce(ctx, &OrderPlaced{OrderId: fmt.Sprintf("ord-%d", i)}, nil); err != nil {
			return err
		}
	}
	return nil
}

// recordOrder is slow so the backlog drains over ~50s, longer than the
// collector's 30s poll, and the two backlog numbers printBacklog reads diverge.
func recordOrder(ctx context.Context, order *OrderPlaced) error {
	time.Sleep(10 * time.Second)
	return nil
}

// printBacklog prints the group's live backlog beside its last collected one.
func printBacklog(ctx context.Context, ledgerMetrics *vulkan.ConsumerMetricsHandle) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}

		snapshot, err := ledgerMetrics.Snapshot(ctx)
		if err != nil {
			return err
		}
		live := snapshot.Cursor.Backlog

		collected, err := ledgerMetrics.CursorBacklog().Latest(ctx)
		if err != nil {
			return err
		}
		if collected == nil {
			fmt.Printf("live backlog %d, nothing collected yet\n", live)
			continue
		}
		fmt.Printf("live backlog %d, collected backlog %g as of %s ago\n",
			live, collected.Value, time.Since(collected.At).Round(time.Second))
	}
}
