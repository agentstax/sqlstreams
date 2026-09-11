package main

// log compaction + retention e2e test: does 8a's retention correctly garbage
// collect compaction_head when it reaps a compacted key's last surviving row?
//
// Two scenarios, one per janitor path a stream's PartitionSize routes it
// through:
//   - dropPartition: a small PartitionSize rolls a dormant key's sole
//     partition out of active use and past ttl; DropExpiredPartitions
//     removes the whole partition and must take compaction_head's now-dangling
//     pointer with it.
//   - sweepBatch: a large PartitionSize keeps everything in partition 0
//     forever; SweepExpiredPartitions reaps the individually-expired row
//     from the front and must do the identical compaction_head cleanup.
//
// A key touched again inside the ttl window proves the opposite case in
// each scenario too: retention doing nothing to a key that's still alive --
// this is intentional expiration, not compaction-awareness bolted onto
// retention (decision record [0269] in .docs/decisions/).

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
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	ttl       = 100 * time.Millisecond
	ttlMargin = 300 * time.Millisecond
	batchSize = 1000
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

	dropPartitionScenario(ctx, pool)
	sweepBatchScenario(ctx, pool)

	fmt.Println("\n✅ LATEST KEYS RETENTION E2E TEST PASSED")
	fmt.Println("   a dormant key's last row aging out takes its compaction_head pointer with it,")
	fmt.Println("   exactly like Kafka's own cleanup.policy=compact,delete -- a key touched")
	fmt.Println("   inside the ttl window survives every pass untouched, either path.")
	return nil
}

func dropPartitionScenario(ctx context.Context, pool *pgxpool.Pool) {
	step("dropPartition: a whole-partition rollover reaps a dormant key's last row")

	const partitionSize = int64(4)
	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	common.Must(err)

	ds, err := iDatastore.NewPostgresDatastore(ctx, pool, nil)
	common.Must(err)

	streamName := fmt.Sprintf("phase8c.compactionheadretention.drop.%d", time.Now().UnixNano())
	tp, err := client.Stream[sqlstreams.RawPayload](streamName).Register(ctx, &sqlstreams.StreamConfig{PartitionSize: partitionSize})
	common.Must(err)
	defer func() {
		common.Must(client.Stream[sqlstreams.RawPayload](streamName).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	}()

	wpInstance, err := client.Stream[common.Work](tp.Name).Producer().Register(ctx, nil)
	common.Must(err)
	janitorDatastore, err := janitordatastore.NewJanitorDatastore(ds, ds.Logger)
	common.Must(err)

	// fill partition 0 with a dormant key + filler, then age past ttl
	publish(ctx, wpInstance, "dormant-key")
	publish(ctx, wpInstance, "")
	publish(ctx, wpInstance, "")
	publish(ctx, wpInstance, "")
	time.Sleep(ttl + ttlMargin)

	// roll into partition 1 so partition 0 is no longer active
	publish(ctx, wpInstance, "alive-key")
	publish(ctx, wpInstance, "")
	publish(ctx, wpInstance, "")
	publish(ctx, wpInstance, "")

	assertLatestExists(ctx, ds, tp.Id, "dormant-key", true)
	assertLatestExists(ctx, ds, tp.Id, "alive-key", true)

	common.Must(janitorDatastore.DropExpiredPartitions(ctx, tp.Id, partitionSize, ttl, true, tp.DeliveryLogMode))

	assertLatestExists(ctx, ds, tp.Id, "dormant-key", false)
	assertLatestExists(ctx, ds, tp.Id, "alive-key", true)
}

func sweepBatchScenario(ctx context.Context, pool *pgxpool.Pool) {
	step("sweepBatch: a low-volume tail reaps a dormant key's last row individually")

	const partitionSize = int64(1000000) // matches migration 001's original width -- never rolls
	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	common.Must(err)

	ds, err := iDatastore.NewPostgresDatastore(ctx, pool, nil)
	common.Must(err)

	streamName := fmt.Sprintf("phase8c.compactionheadretention.sweep.%d", time.Now().UnixNano())
	tp, err := client.Stream[sqlstreams.RawPayload](streamName).Register(ctx, &sqlstreams.StreamConfig{PartitionSize: partitionSize})
	common.Must(err)
	defer func() {
		common.Must(client.Stream[sqlstreams.RawPayload](streamName).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	}()

	wpInstance, err := client.Stream[common.Work](tp.Name).Producer().Register(ctx, nil)
	common.Must(err)
	janitorDatastore, err := janitordatastore.NewJanitorDatastore(ds, ds.Logger)
	common.Must(err)

	publish(ctx, wpInstance, "dormant-key")
	time.Sleep(ttl + ttlMargin)
	publish(ctx, wpInstance, "alive-key") // well inside ttl

	assertLatestExists(ctx, ds, tp.Id, "dormant-key", true)
	assertLatestExists(ctx, ds, tp.Id, "alive-key", true)

	common.Must(janitorDatastore.SweepExpiredPartitions(ctx, tp.Id, partitionSize, ttl, 0, true, batchSize, tp.DeliveryLogMode))

	assertLatestExists(ctx, ds, tp.Id, "dormant-key", false)
	assertLatestExists(ctx, ds, tp.Id, "alive-key", true)

	// sweep repeatedly -- a key kept alive inside ttl survives every pass, not just the first
	for range 3 {
		publish(ctx, wpInstance, "alive-key")
		time.Sleep(ttl / 4)
		common.Must(janitorDatastore.SweepExpiredPartitions(ctx, tp.Id, partitionSize, ttl, 0, true, batchSize, tp.DeliveryLogMode))
	}
	assertLatestExists(ctx, ds, tp.Id, "alive-key", true)
}

// ---- helpers ----

func publish(ctx context.Context, wpInstance *sqlstreams.ProducerInstance[common.Work], key string) {
	opts := &sqlstreams.ProduceOptions{}
	if key != "" {
		opts.MessageKey = key
		opts.Compaction = &sqlstreams.CompactionOptions{Enable: true}
	}
	_, err := wpInstance.ProduceFunc(ctx, func(ctx context.Context, tx sqlstreams.Tx) (*common.Work, error) {
		return common.NewWork(30, "admin@example.com")
	}, opts)
	common.Must(err)
}

func assertLatestExists(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64, key string, want bool) {
	var count int
	common.Must(ds.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE compaction_key=$1;`, ds.Schema, stream.CompactionHeadTable(streamId)), key).Scan(&count))
	got := count > 0
	if got != want {
		common.Die(fmt.Sprintf("compaction_head[%s] exists=%v, want %v", key, got, want))
	}
	fmt.Printf("  ✓ compaction_head[%s] exists=%v\n", key, got)
}

func step(s string) { fmt.Printf("\n--- %s ---\n", s) }
