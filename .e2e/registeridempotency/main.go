package main

// register idempotency e2e test: re-registering a topic resolves to the same row,
// and the newest declaration's mutable config replaces what is stored -- guarding
// registerTopic's found path (replaceConfig).
// Each run uses an isolated installation to check validation before bootstrap
// and preservation of a customized system declaration.
//
// Confirms:
//  1. first Register creates the topic and appends its first topic_config_log row.
//  2. re-registering the SAME config resolves to the same topic, no error and
//     no write -- topic_config_log gains nothing.
//  3. re-registering DIFFERENT mutable config keeps the id, replaces the
//     values, and appends the new snapshot to topic_config_log.
//  4. re-registering a different PartitionSize returns ErrTopicConfigMismatch:
//     message_log's partition boundaries are derived from it.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/agentstax/vulkan/pkg/alert/partitioncount"
	iDatastore "github.com/agentstax/vulkan/pkg/datastore"
	"github.com/agentstax/vulkan/pkg/topic"
	vulkan "github.com/agentstax/vulkan/pkg/vulkan"
)

func main() {
	if err := run(); err != nil {
		fmt.Printf("\n❌ E2E TEST FAILED: %s\n", err.Error())
		os.Exit(1)
	}
}

// testFailure is what die panics with; run recovers it into its error so
// main's deferred cleanup runs on a failed assertion.
type testFailure struct {
	message string
}

func (f testFailure) Error() string {
	return f.message
}

func run() (err error) {
	defer func() {
		switch recovered := recover().(type) {
		case nil:
		case testFailure:
			err = recovered
		default:
			panic(recovered)
		}
	}()
	ctx := context.Background()

	pool, err := vulkan.NewPostgresPool(ctx, "example_user", "example_password", "localhost", "example_db", nil)
	must(err)
	defer pool.Close()

	schema := fmt.Sprintf("registeridempotency_%d", time.Now().UnixNano())
	client, err := vulkan.NewClient(ctx, pool, &vulkan.ClientConfig{Schema: schema, AllowDestroy: true})
	must(err)
	defer func() {
		must(client.System().Destroy(ctx, &vulkan.DestroyOptions{Force: true}))
		_, err := pool.Exec(ctx, fmt.Sprintf(`
			-- vulkan: registeridempotency.run
			DROP SCHEMA IF EXISTS %[1]s;
		`, schema))
		must(err)
	}()
	ds, err := iDatastore.NewPostgresDatastore(ctx, pool, &iDatastore.PostgresDatastoreConfig{Schema: schema})
	must(err)

	name := fmt.Sprintf("registeridempotency.e2e.%d", time.Now().UnixNano())
	topicHandle := client.Topic[vulkan.RawPayload](name)

	step("invalid names leave an unregistered installation untouched")
	for _, invalidName := range []string{"", "Orders!", "orders.*", "__system.evil"} {
		_, err := client.Topic[vulkan.RawPayload](invalidName).Register(ctx, nil)
		if err == nil {
			die(fmt.Sprintf("name %q must be rejected", invalidName))
		}
		var exists bool
		must(pool.QueryRow(ctx, `
			-- vulkan: registeridempotency.run
			SELECT to_regnamespace($1) IS NOT NULL;
		`, schema).Scan(&exists))
		if exists {
			die(fmt.Sprintf("name %q created system resources", invalidName))
		}
	}

	step("invalid configs leave an unregistered installation untouched")
	for field, cfg := range map[string]*vulkan.TopicConfig{
		"PartitionSize":          {PartitionSize: 1},
		"RetentionTTL":           {RetentionTTL: -time.Second},
		"IdempotencyKeyTTL":      {IdempotencyKeyTTL: -time.Second},
		"EmptyCompactionHeadTTL": {EmptyCompactionHeadTTL: -time.Second},
		"DeliveryLogMode":        {DeliveryLogMode: "unsupported"},
	} {
		_, err := topicHandle.Register(ctx, cfg)
		if err == nil {
			die(fmt.Sprintf("%s must be rejected", field))
		}
		var exists bool
		must(pool.QueryRow(ctx, `
			-- vulkan: registeridempotency.run
			SELECT to_regnamespace($1) IS NOT NULL;
		`, schema).Scan(&exists))
		if exists {
			die(fmt.Sprintf("%s created system resources", field))
		}
	}

	step("first register creates the topic")
	created, err := topicHandle.Register(ctx, &vulkan.TopicConfig{RetentionTTL: 720 * time.Hour})
	must(err)
	system, err := client.System().Get(ctx)
	must(err)
	if system == nil || system.Id != created.SystemId {
		die("valid topic registration must bootstrap its system")
	}
	if count := topicLogCount(ctx, ds, created.Id); count != 1 {
		die(fmt.Sprintf("topic_config_log rows after create = %d, want 1", count))
	}
	if created.EmptyCompactionHeadTTL != time.Hour {
		die(fmt.Sprintf("default EmptyCompactionHeadTTL = %v, want 1h", created.EmptyCompactionHeadTTL))
	}
	if ttl := topicLogEmptyCompactionHeadTTL(ctx, ds, created.Id); ttl != time.Hour {
		die(fmt.Sprintf("topic_config_log EmptyCompactionHeadTTL after create = %v, want 1h", ttl))
	}
	fmt.Printf("  ✓ created id=%d, first topic_config_log row appended\n", created.Id)

	step("re-register SAME config is idempotent, not a mismatch")
	// Fresh Config with the identical caller-set field -- RegisterTopic mutates
	// what it's given via WithDefaults, so don't reuse the first one.
	again, err := client.Topic[vulkan.RawPayload](name).Register(ctx, &vulkan.TopicConfig{RetentionTTL: 720 * time.Hour})
	if err != nil {
		die(fmt.Sprintf("re-register with identical config must succeed, got: %v", err))
	}
	if again.Id != created.Id {
		die(fmt.Sprintf("re-register resolved a different id: got %d, want %d", again.Id, created.Id))
	}
	if count := topicLogCount(ctx, ds, created.Id); count != 1 {
		die(fmt.Sprintf("topic_config_log rows after a no-change register = %d, want 1", count))
	}
	fmt.Printf("  ✓ re-register resolved same id=%d, no mismatch, nothing appended\n", again.Id)

	step("re-register DIFFERENT config replaces the stored mutable config")
	redeclared, err := client.Topic[vulkan.RawPayload](name).Register(ctx, &vulkan.TopicConfig{
		RetentionTTL:           168 * time.Hour,
		EmptyCompactionHeadTTL: 2 * time.Hour,
	})
	must(err)
	if redeclared.Id != created.Id {
		die(fmt.Sprintf("re-declare resolved a different id: got %d, want %d", redeclared.Id, created.Id))
	}
	if redeclared.RetentionTTL != 168*time.Hour {
		die(fmt.Sprintf("re-declared RetentionTTL = %v, want 168h", redeclared.RetentionTTL))
	}
	if redeclared.EmptyCompactionHeadTTL != 2*time.Hour {
		die(fmt.Sprintf("re-declared EmptyCompactionHeadTTL = %v, want 2h", redeclared.EmptyCompactionHeadTTL))
	}
	if count := topicLogCount(ctx, ds, created.Id); count != 2 {
		die(fmt.Sprintf("topic_config_log rows after a config change = %d, want 2", count))
	}
	if ttl := topicLogEmptyCompactionHeadTTL(ctx, ds, created.Id); ttl != 2*time.Hour {
		die(fmt.Sprintf("topic_config_log EmptyCompactionHeadTTL after replace = %v, want 2h", ttl))
	}
	fmt.Printf("  ✓ newest declaration won: retention now %v on the same id=%d, snapshot appended\n", redeclared.RetentionTTL, redeclared.Id)

	step("re-register DIFFERENT PartitionSize is rejected")
	_, err = client.Topic[vulkan.RawPayload](name).Register(ctx, &vulkan.TopicConfig{
		PartitionSize:          created.PartitionSize + 1,
		RetentionTTL:           168 * time.Hour,
		EmptyCompactionHeadTTL: 2 * time.Hour,
	})
	if !errors.Is(err, topic.ErrTopicConfigMismatch) {
		die(fmt.Sprintf("re-register with a different PartitionSize must return ErrTopicConfigMismatch, got: %v", err))
	}
	fmt.Printf("  ✓ changed PartitionSize rejected with ErrTopicConfigMismatch\n")

	step("nil topic config preserves a customized system declaration")
	must(client.System().Register(ctx, &vulkan.SystemConfig{
		PartitionCountAlert: &vulkan.PartitionCountAlertConfig{ScheduleExpression: "@daily"},
	}))
	before, err := client.Scheduler(partitioncount.JobName).Get(ctx)
	must(err)
	if before == nil || before.Expression != "@daily" {
		die("custom system alert schedule must be installed")
	}
	defaultTopic := client.Topic[vulkan.RawPayload](name + ".defaults")
	defaults, err := defaultTopic.Register(ctx, nil)
	must(err)
	if defaults.PartitionSize != 1_000_000 || defaults.EmptyCompactionHeadTTL != time.Hour {
		die("nil topic config must use defaults")
	}
	after, err := client.Scheduler(partitioncount.JobName).Get(ctx)
	must(err)
	if !reflect.DeepEqual(after, before) {
		die("topic registration must preserve the custom system declaration")
	}

	fmt.Printf("\n✅ register idempotency e2e test PASSED\n")
	return nil
}

// topicLogCount reads the topic's trail directly -- machinery never reads
// topic_config_log, so the e2e test asserts on the table itself.
func topicLogCount(ctx context.Context, ds *iDatastore.PostgresDatastore, topicId int64) int {
	var count int
	must(ds.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.topic_config_log WHERE topic_id = $1;`, ds.Schema), topicId).Scan(&count))
	return count
}

func topicLogEmptyCompactionHeadTTL(ctx context.Context, ds *iDatastore.PostgresDatastore, topicId int64) time.Duration {
	var ttlNs int64
	must(ds.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT empty_compaction_head_ttl_ns FROM %s.topic_config_log WHERE topic_id = $1 ORDER BY id DESC LIMIT 1;`, ds.Schema), topicId).Scan(&ttlNs))
	return time.Duration(ttlNs)
}

func step(s string) { fmt.Printf("\n--- %s ---\n", s) }
func must(err error) {
	if err != nil {
		die(err.Error())
	}
}
func die(msg string) {
	panic(testFailure{message: msg})
}
