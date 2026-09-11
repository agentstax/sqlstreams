package main

// destroy-system e2e test: DestroySystem is RegisterSystem's inverse (decision
// record [0514]). Walks the verb through its guards and its teardown:
//
//   - a registered user stream refuses the destroy (ErrStreamsRegistered)
//   - a running consumer refuses it first (ErrSystemLive) -- the worker
//     guard outranks the stream guard
//   - with the consumer stopped and the user stream destroyed, the unforced
//     destroy succeeds: every control-plane table and every system stream's
//     physical tables are gone; a second destroy is a no-op (idempotent)
//   - RegisterSystem stands the schema back up, leaving the database usable
//
// Self-seeded, self-verifying; ends with the system re-registered.

import (
	"context"
	"errors"
	"fmt"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"os"
	"time"

	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/agentstax/sqlstreams/pkg/system"
)

// every table createSystemTables creates -- the teardown assertion list
var controlPlaneTables = []string{
	"system_config",
	"stream_config",
	"stream_config_log",
	"consumer_group_config",
	"worker_config",
	"worker_config_log",
	"worker_instance",
	"schedule_config",
	"schedule_cursor",
	"migration_log",
}

var ds *iDatastore.PostgresDatastore

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
	ds, err = iDatastore.NewPostgresDatastore(ctx, pool, nil)
	common.Must(err)
	common.Must(client.System().Register(ctx, nil))

	step("seed a user stream with messages")
	streamName := fmt.Sprintf("destroysystem.%d", time.Now().UnixNano())
	tp, err := client.Stream[sqlstreams.RawPayload](streamName).Register(ctx, nil)
	common.Must(err)
	wpInstance, err := client.Stream[common.Work](tp.Name).Producer().Register(ctx, nil)
	common.Must(err)
	for range 3 {
		_, err := wpInstance.ProduceFunc(ctx, func(ctx context.Context, tx sqlstreams.Tx) (*common.Work, error) {
			return common.NewWork(30, "admin@example.com")
		}, nil)
		common.Must(err)
	}

	step("a registered user stream refuses the destroy")
	err = client.System().Destroy(ctx, nil)
	assertErrorIs("ErrStreamsRegistered", err, system.ErrStreamsRegistered)

	step("a running consumer refuses it first -- the worker guard outranks the stream guard")
	wcInstance, err := client.Stream[common.Work](tp.Name).Consumer("destroysystem-group").Register(ctx, nil)
	common.Must(err)
	consumeCtx, stopConsumer := context.WithCancel(ctx)
	consumeDone := make(chan error, 1)
	go func() {
		consumeDone <- wcInstance.Consume(consumeCtx, func(ctx context.Context, work *common.Work) error {
			return nil
		}, &sqlstreams.ConsumeOptions{
			ClaimPollRate: 500 * time.Millisecond,
			InstanceTTL:   2 * time.Second,
		})
	}()
	waitLiveInstances(ctx, true)

	err = client.System().Destroy(ctx, nil)
	assertErrorIs("ErrSystemLive", err, system.ErrSystemLive)

	stopConsumer()
	common.Must(<-consumeDone)
	waitLiveInstances(ctx, false)

	step("consumer stopped: the stream guard is back")
	err = client.System().Destroy(ctx, nil)
	assertErrorIs("ErrStreamsRegistered", err, system.ErrStreamsRegistered)

	step("user stream destroyed: the unforced destroy succeeds")
	// a system stream's id, so the teardown assert can cover a physical
	// table the destroy itself must drop (not one DestroyStream already took)
	var alertsStreamId int64
	common.Must(ds.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.stream_config WHERE name = '__system.alerts';`, ds.Schema)).Scan(&alertsStreamId))

	common.Must(client.Stream[sqlstreams.RawPayload](streamName).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
	common.Must(client.System().Destroy(ctx, nil))

	for _, table := range controlPlaneTables {
		assertTableExists(ctx, ds.Schema+"."+table, false)
	}
	assertTableExists(ctx, fmt.Sprintf("%s.%s", ds.Schema, stream.MessageLogTable(alertsStreamId)), false)

	step("a second destroy is a no-op, not an error")
	common.Must(client.System().Destroy(ctx, nil))
	fmt.Println("  ✓ destroy of an already-destroyed system returned nil")

	step("RegisterSystem stands the schema back up")
	common.Must(client.System().Register(ctx, nil))
	for _, table := range controlPlaneTables {
		assertTableExists(ctx, ds.Schema+"."+table, true)
	}
	var streamCount int
	common.Must(ds.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stream_config;`, ds.Schema)).Scan(&streamCount))
	assertTrue(fmt.Sprintf("the 3 system streams re-registered (got %d)", streamCount), streamCount == 3)

	fmt.Println("\n✅ DESTROY SYSTEM E2E TEST PASSED")
	fmt.Println("   guards refuse while workers run or streams remain; the unforced destroy")
	fmt.Println("   returns the database to its pre-register state, and RegisterSystem rebuilds it.")
	return nil
}

// ---- helpers ----

// waitLiveInstances polls until some worker_instance row is live (want=true)
// or every row is gone or expired (want=false). Released instances delete
// their rows; a crashed one lingers only until the e2e test's 2s InstanceTTL.
func waitLiveInstances(ctx context.Context, want bool) {
	deadline := time.Now().Add(30 * time.Second)
	for {
		var live bool
		common.Must(ds.Pool.QueryRow(ctx, fmt.Sprintf(`
			SELECT EXISTS (SELECT 1 FROM %s.worker_instance WHERE expires_at > now());
		`, ds.Schema)).Scan(&live))
		if live == want {
			return
		}
		if time.Now().After(deadline) {
			common.Die(fmt.Sprintf("live worker instances never became %v", want))
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func assertTableExists(ctx context.Context, table string, want bool) {
	var exists bool
	common.Must(ds.Pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL;`, table).Scan(&exists))
	if exists != want {
		common.Die(fmt.Sprintf("table %s exists=%v, want %v", table, exists, want))
	}
	fmt.Printf("  ✓ table %s exists=%v\n", table, exists)
}

func assertErrorIs(label string, err error, target error) {
	if !errors.Is(err, target) {
		common.Die(fmt.Sprintf("%s: got %v", label, err))
	}
	fmt.Printf("  ✓ refused with %s: %v\n", label, err)
}

func assertTrue(label string, cond bool) {
	if !cond {
		common.Die(fmt.Sprintf("%s: got false, want true", label))
	}
	fmt.Printf("  ✓ %s\n", label)
}

func step(s string) { fmt.Printf("\n--- %s ---\n", s) }
