package main

// Phase 8a e2e test (c): the low-volume tail -- a partition that never fills wide
// enough to earn a whole-partition drop still needs its expired rows to leave.
//
// Registers its own stream at the real migration-shipped partition width
// (1,000,000), destroyed on exit -- staying under that width (never rolling to
// a second partition) is exactly the condition the sweep exists to cover, so no
// schema swap is needed, unlike partition/dropfloor. A dedicated stream
// also means this e2e test's own cursorFloor is isolated from every other e2e test and
// group sharing the dev DB, so unlike the pre-8b version it no longer needs to
// force AllowDropPastCommitted=true just to dodge a floor some unrelated
// group's leftover state might be pinning.
//
// Confirms: DropExpiredPartitions is a no-op here (the stream's first partition
// is still active, nowhere near partitionSize, so the whole-partition path
// never engages at this volume) while SweepExpiredPartitions deletes exactly
// the expired prefix and leaves the fresher rows and the partition itself
// untouched.

import (
	"context"
	"fmt"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"os"
	"time"

	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
	janitordatastore "github.com/agentstax/sqlstreams/pkg/stream/janitor/controller/datastore"
)

const (
	partitionSize = int64(1000000) // matches migration 001's original message_log_0 width -- no schema swap this e2e test
	ttl           = 100 * time.Millisecond
	ttlMargin     = 300 * time.Millisecond
	batchSize     = 1000
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

	pool, err := common.NewPool(ctx, nil)
	common.Must(err)
	defer pool.Close()

	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	common.Must(err)
	ds, err := iDatastore.NewPostgresDatastore(ctx, pool, nil)
	common.Must(err)

	streamName := fmt.Sprintf("phase8a.sweep.%d", time.Now().UnixNano())
	tp, err := client.Stream[sqlstreams.RawPayload](streamName).Register(ctx, &sqlstreams.StreamConfig{PartitionSize: partitionSize})
	common.Must(err)
	defer func() {
		common.Must(client.Stream[sqlstreams.RawPayload](streamName).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	}()

	wpInstance, err := client.Stream[common.Work](tp.Name).Producer().Register(ctx, nil)
	common.Must(err)
	janitorDatastore, err := janitordatastore.NewJanitorDatastore(ds, ds.Logger)
	common.Must(err)

	step("publish 4 'old' messages, then let them age past ttl")
	head0 := head(ctx, ds, tp.Id)
	for range 4 {
		publish(ctx, wpInstance)
	}
	oldLow, oldHigh := head0, head0+4
	time.Sleep(ttl + ttlMargin)

	step("publish 3 'fresh' messages -- well inside ttl")
	freshLow, freshHigh := head(ctx, ds, tp.Id), head(ctx, ds, tp.Id)+3
	for range 3 {
		publish(ctx, wpInstance)
	}
	fmt.Printf("  old ids (%d,%d], fresh ids (%d,%d]\n", oldLow, oldHigh, freshLow, freshHigh)

	step("DropExpiredPartitions -- no-op, the stream's first partition is still active at this volume")
	common.Must(janitorDatastore.DropExpiredPartitions(ctx, tp.Id, partitionSize, ttl, true, tp.DeliveryLogMode))
	assertInt("partition 0 survives", partitionCount(ctx, ds, tp.Id), 1)
	assertInt("old rows untouched by drop", countInRange(ctx, ds, tp.Id, oldLow, oldHigh), 4)
	assertInt("fresh rows untouched by drop", countInRange(ctx, ds, tp.Id, freshLow, freshHigh), 3)

	step("SweepExpiredPartitions -- deletes exactly the expired prefix")
	common.Must(janitorDatastore.SweepExpiredPartitions(ctx, tp.Id, partitionSize, ttl, 0, true, batchSize, tp.DeliveryLogMode))
	assertInt("old rows swept", countInRange(ctx, ds, tp.Id, oldLow, oldHigh), 0)
	assertInt("fresh rows survive -- not yet past ttl", countInRange(ctx, ds, tp.Id, freshLow, freshHigh), 3)
	assertInt("partition 0 itself survives -- sweep deletes rows, not partitions", partitionCount(ctx, ds, tp.Id), 1)

	fmt.Println("\n✅ SWEEP E2E TEST PASSED")
	fmt.Println("   a partition too low-volume to ever earn a whole-partition drop still sheds its")
	fmt.Println("   expired prefix via the sweep -- drop and sweep cover each other's weak end.")
	return nil
}

// ---- helpers ----

func publish(ctx context.Context, wpInstance *sqlstreams.ProducerInstance[common.Work]) {
	_, err := wpInstance.ProduceFunc(ctx, func(ctx context.Context, tx sqlstreams.Tx) (*common.Work, error) {
		return common.NewWork(30, "admin@example.com")
	}, nil)
	common.Must(err)
}

func head(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64) int64 {
	return scalar(ctx, ds, fmt.Sprintf(`SELECT COALESCE(MAX(id), 0) FROM %s.%s`, ds.Schema, stream.MessageLogTable(streamId)))
}

func countInRange(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId, low, high int64) int64 {
	return scalar(ctx, ds, fmt.Sprintf(`SELECT count(*) FROM %s.%s WHERE id > $1 AND id <= $2`, ds.Schema, stream.MessageLogTable(streamId)), low, high)
}

func partitionCount(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64) int64 {
	return scalar(ctx, ds, fmt.Sprintf(`
		SELECT count(*) FROM pg_inherits i
		JOIN pg_class c ON c.oid = i.inhrelid
		WHERE i.inhparent = '%s.%s'::regclass;
	`, ds.Schema, stream.MessageLogTable(streamId)))
}

func scalar(ctx context.Context, ds *iDatastore.PostgresDatastore, q string, args ...any) int64 {
	var v int64
	common.Must(ds.Pool.QueryRow(ctx, q, args...).Scan(&v))
	return v
}

func step(s string) { fmt.Printf("\n--- %s ---\n", s) }
func assertInt(label string, got, want int64) {
	if got != want {
		common.Die(fmt.Sprintf("%s: got %d, want %d", label, got, want))
	}
	fmt.Printf("  ✓ %s (%d)\n", label, got)
}
