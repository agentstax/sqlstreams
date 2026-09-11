package main

// register idempotency e2e test: re-registering a stream resolves to the same row,
// and the newest declaration's mutable config replaces what is stored -- guarding
// registerStream's found path (replaceConfig).
// Each run uses an isolated installation to check validation before bootstrap
// and preservation of a customized system declaration.
//
// Confirms:
//  1. first Register creates the stream and appends its first stream_config_log row.
//  2. re-registering the SAME config resolves to the same stream, no error and
//     no write -- stream_config_log gains nothing.
//  3. re-registering DIFFERENT mutable config keeps the id, replaces the
//     values, and appends the new snapshot to stream_config_log.
//  4. re-registering a different PartitionSize returns ErrStreamConfigMismatch:
//     message_log's partition boundaries are derived from it.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	"github.com/agentstax/sqlstreams/pkg/alert/partitioncount"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
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

	pool, err := common.NewPool(ctx, nil)
	common.Must(err)
	defer pool.Close()

	schema := fmt.Sprintf("registeridempotency_%d", time.Now().UnixNano())
	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{Schema: schema, AllowDestroy: true})
	common.Must(err)
	defer func() {
		common.Must(client.System().Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))
		_, err := pool.Exec(ctx, fmt.Sprintf(`
			-- sqlstreams: registeridempotency.run
			DROP SCHEMA IF EXISTS %[1]s;
		`, schema))
		common.Must(err)
	}()
	ds, err := iDatastore.NewPostgresDatastore(ctx, pool, &iDatastore.PostgresDatastoreConfig{Schema: schema})
	common.Must(err)

	name := fmt.Sprintf("registeridempotency.e2e.%d", time.Now().UnixNano())
	streamHandle := client.Stream[sqlstreams.RawPayload](name)

	step("invalid names leave an unregistered installation untouched")
	for _, invalidName := range []string{"", "Orders!", "orders.*", "__system.evil"} {
		_, err := client.Stream[sqlstreams.RawPayload](invalidName).Register(ctx, nil)
		if err == nil {
			common.Die(fmt.Sprintf("name %q must be rejected", invalidName))
		}
		var exists bool
		common.Must(pool.QueryRow(ctx, `
			-- sqlstreams: registeridempotency.run
			SELECT to_regnamespace($1) IS NOT NULL;
		`, schema).Scan(&exists))
		if exists {
			common.Die(fmt.Sprintf("name %q created system resources", invalidName))
		}
	}

	step("invalid configs leave an unregistered installation untouched")
	for field, cfg := range map[string]*sqlstreams.StreamConfig{
		"PartitionSize":          {PartitionSize: 1},
		"RetentionTTL":           {RetentionTTL: -time.Second},
		"IdempotencyKeyTTL":      {IdempotencyKeyTTL: -time.Second},
		"EmptyCompactionHeadTTL": {EmptyCompactionHeadTTL: -time.Second},
		"DeliveryLogMode":        {DeliveryLogMode: "unsupported"},
	} {
		_, err := streamHandle.Register(ctx, cfg)
		if err == nil {
			common.Die(fmt.Sprintf("%s must be rejected", field))
		}
		var exists bool
		common.Must(pool.QueryRow(ctx, `
			-- sqlstreams: registeridempotency.run
			SELECT to_regnamespace($1) IS NOT NULL;
		`, schema).Scan(&exists))
		if exists {
			common.Die(fmt.Sprintf("%s created system resources", field))
		}
	}

	step("first register creates the stream")
	created, err := streamHandle.Register(ctx, &sqlstreams.StreamConfig{RetentionTTL: 720 * time.Hour})
	common.Must(err)
	system, err := client.System().Get(ctx)
	common.Must(err)
	if system == nil || system.Id != created.SystemId {
		common.Die("valid stream registration must bootstrap its system")
	}
	if count := streamLogCount(ctx, ds, created.Id); count != 1 {
		common.Die(fmt.Sprintf("stream_config_log rows after create = %d, want 1", count))
	}
	if created.EmptyCompactionHeadTTL != time.Hour {
		common.Die(fmt.Sprintf("default EmptyCompactionHeadTTL = %v, want 1h", created.EmptyCompactionHeadTTL))
	}
	if ttl := streamLogEmptyCompactionHeadTTL(ctx, ds, created.Id); ttl != time.Hour {
		common.Die(fmt.Sprintf("stream_config_log EmptyCompactionHeadTTL after create = %v, want 1h", ttl))
	}
	fmt.Printf("  ✓ created id=%d, first stream_config_log row appended\n", created.Id)

	step("re-register SAME config is idempotent, not a mismatch")
	// Fresh Config with the identical caller-set field -- RegisterStream mutates
	// what it's given via WithDefaults, so don't reuse the first one.
	again, err := client.Stream[sqlstreams.RawPayload](name).Register(ctx, &sqlstreams.StreamConfig{RetentionTTL: 720 * time.Hour})
	if err != nil {
		common.Die(fmt.Sprintf("re-register with identical config must succeed, got: %v", err))
	}
	if again.Id != created.Id {
		common.Die(fmt.Sprintf("re-register resolved a different id: got %d, want %d", again.Id, created.Id))
	}
	if count := streamLogCount(ctx, ds, created.Id); count != 1 {
		common.Die(fmt.Sprintf("stream_config_log rows after a no-change register = %d, want 1", count))
	}
	fmt.Printf("  ✓ re-register resolved same id=%d, no mismatch, nothing appended\n", again.Id)

	step("re-register DIFFERENT config replaces the stored mutable config")
	redeclared, err := client.Stream[sqlstreams.RawPayload](name).Register(ctx, &sqlstreams.StreamConfig{
		RetentionTTL:           168 * time.Hour,
		EmptyCompactionHeadTTL: 2 * time.Hour,
	})
	common.Must(err)
	if redeclared.Id != created.Id {
		common.Die(fmt.Sprintf("re-declare resolved a different id: got %d, want %d", redeclared.Id, created.Id))
	}
	if redeclared.RetentionTTL != 168*time.Hour {
		common.Die(fmt.Sprintf("re-declared RetentionTTL = %v, want 168h", redeclared.RetentionTTL))
	}
	if redeclared.EmptyCompactionHeadTTL != 2*time.Hour {
		common.Die(fmt.Sprintf("re-declared EmptyCompactionHeadTTL = %v, want 2h", redeclared.EmptyCompactionHeadTTL))
	}
	if count := streamLogCount(ctx, ds, created.Id); count != 2 {
		common.Die(fmt.Sprintf("stream_config_log rows after a config change = %d, want 2", count))
	}
	if ttl := streamLogEmptyCompactionHeadTTL(ctx, ds, created.Id); ttl != 2*time.Hour {
		common.Die(fmt.Sprintf("stream_config_log EmptyCompactionHeadTTL after replace = %v, want 2h", ttl))
	}
	fmt.Printf("  ✓ newest declaration won: retention now %v on the same id=%d, snapshot appended\n", redeclared.RetentionTTL, redeclared.Id)

	step("re-register DIFFERENT PartitionSize is rejected")
	_, err = client.Stream[sqlstreams.RawPayload](name).Register(ctx, &sqlstreams.StreamConfig{
		PartitionSize:          created.PartitionSize + 1,
		RetentionTTL:           168 * time.Hour,
		EmptyCompactionHeadTTL: 2 * time.Hour,
	})
	if !errors.Is(err, stream.ErrStreamConfigMismatch) {
		common.Die(fmt.Sprintf("re-register with a different PartitionSize must return ErrStreamConfigMismatch, got: %v", err))
	}
	fmt.Printf("  ✓ changed PartitionSize rejected with ErrStreamConfigMismatch\n")

	step("nil stream config preserves a customized system declaration")
	common.Must(client.System().Register(ctx, &sqlstreams.SystemConfig{
		PartitionCountAlert: &sqlstreams.PartitionCountAlertConfig{ScheduleExpression: "@daily"},
	}))
	before, err := client.Scheduler(partitioncount.JobName).Get(ctx)
	common.Must(err)
	if before == nil || before.Expression != "@daily" {
		common.Die("custom system alert schedule must be installed")
	}
	defaultStream := client.Stream[sqlstreams.RawPayload](name + ".defaults")
	defaults, err := defaultStream.Register(ctx, nil)
	common.Must(err)
	if defaults.PartitionSize != 1_000_000 || defaults.EmptyCompactionHeadTTL != time.Hour {
		common.Die("nil stream config must use defaults")
	}
	after, err := client.Scheduler(partitioncount.JobName).Get(ctx)
	common.Must(err)
	if !reflect.DeepEqual(after, before) {
		common.Die("stream registration must preserve the custom system declaration")
	}

	fmt.Printf("\n✅ register idempotency e2e test PASSED\n")
	return nil
}

// streamLogCount reads the stream's trail directly -- machinery never reads
// stream_config_log, so the e2e test asserts on the table itself.
func streamLogCount(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64) int {
	var count int
	common.Must(ds.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.stream_config_log WHERE stream_id = $1;`, ds.Schema), streamId).Scan(&count))
	return count
}

func streamLogEmptyCompactionHeadTTL(ctx context.Context, ds *iDatastore.PostgresDatastore, streamId int64) time.Duration {
	var ttlNs int64
	common.Must(ds.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT empty_compaction_head_ttl_ns FROM %s.stream_config_log WHERE stream_id = $1 ORDER BY id DESC LIMIT 1;`, ds.Schema), streamId).Scan(&ttlNs))
	return time.Duration(ttlNs)
}

func step(s string) { fmt.Printf("\n--- %s ---\n", s) }
