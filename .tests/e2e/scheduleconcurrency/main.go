package main

// Schedule concurrency e2e test: SchedulerInstance.Schedule(ctx) and
// SchedulerInstance.Schedule(ctx) and Client.Manager().Run(ctx) run the system manager, so two concurrent runs in
// one process are both admitted and the manager row's claim gate is what
// admits one reconcile loop between them. Also proves the Scheduler handle's
// Get reads the declared row.

import (
	"context"
	"fmt"
	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	"github.com/agentstax/sqlstreams/pkg/datastore"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"time"

	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
)

type ReportRequestedV1 struct {
	Kind string `json:"kind"`
}

func (ReportRequestedV1) SchemaVersion() int { return 1 }

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

	streamName := fmt.Sprintf("scheduleconcurrency.reports.%d", run)
	_, err = client.Stream[sqlstreams.RawPayload](streamName).Register(ctx, nil)
	common.Must(err)

	step("RegisterSchedule returns an instance; Scheduler.Get reads the row")
	scheduleName := fmt.Sprintf("scheduleconcurrency.nightly.%d", run)
	nightly, err := client.Scheduler(scheduleName).Register[ReportRequestedV1](ctx, streamName, "0 3 * * *", &ReportRequestedV1{Kind: "nightly"}, nil)
	common.Must(err)
	row, err := client.Scheduler(scheduleName).Get(ctx)
	common.Must(err)
	if row == nil {
		common.Die("expected the declared schedule row, got nil")
	}
	assertString("schedule name", row.Name, scheduleName)

	step("an instance Schedule and RunManager are both admitted")
	runCtx, stopRun := context.WithCancel(ctx)
	defer stopRun()
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- nightly.Schedule(runCtx)
	}()

	secondCtx, stopSecond := context.WithCancel(ctx)
	defer stopSecond()
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- client.Manager().Run(secondCtx)
	}()

	time.Sleep(5 * time.Second)
	select {
	case err := <-firstDone:
		common.Die(fmt.Sprintf("first scheduler instance run exited early: %v", err))
	case err := <-secondDone:
		common.Die(fmt.Sprintf("second manager run was refused: %v", err))
	default:
	}
	live := scalar(ctx, pool, `
		SELECT count(*)
		FROM %[1]s.worker_instance i
		JOIN %[1]s.worker_config w ON w.id = i.worker_id
		WHERE w.name = 'manager'
			AND w.system_id IS NOT NULL
			AND i.expires_at > now()`)
	if live != 1 {
		common.Die(fmt.Sprintf("%d live system manager instances, want 1 -- the row's claim gate admits one, so an installation created before the gate (target_instances -1) needs a drop+recreate of its schema", live))
	}
	fmt.Println("  ✓ both runs admitted, one live manager instance between them")

	step("each run stops clean on its own ctx")
	stopSecond()
	if err := <-secondDone; err != nil {
		common.Die(fmt.Sprintf("second manager run: expected nil on requested stop, got %v", err))
	}
	stopRun()
	if err := <-firstDone; err != nil {
		common.Die(fmt.Sprintf("first scheduler instance run: expected nil on requested stop, got %v", err))
	}
	fmt.Println("  ✓ both returned nil on a requested stop")

	step("cleanup")
	common.Must(client.Scheduler(scheduleName).Destroy(ctx))
	common.Must(client.Stream[sqlstreams.RawPayload](streamName).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))

	fmt.Println("\n✅ SCHEDULE CONCURRENCY E2E TEST PASSED")
	return nil
}

// scalar runs a one-value query whose every table name is the client's own
// schema at verb [1].
func scalar(ctx context.Context, pool *pgxpool.Pool, sql string) int64 {
	var value int64
	common.Must(pool.QueryRow(ctx, fmt.Sprintf(sql, datastore.DefaultSchema)).Scan(&value))
	return value
}

func assertString(label string, got string, want string) {
	if got != want {
		common.Die(fmt.Sprintf("%s: got %q, want %q", label, got, want))
	}
	fmt.Printf("  ✓ %s (%q)\n", label, got)
}

func step(s string) { fmt.Printf("\n--- %s ---\n", s) }
