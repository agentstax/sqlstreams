package vulkan

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/agentstax/vulkan/pkg/metrics"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAlertSnapshot(t *testing.T) {
	url := os.Getenv("VULKAN_CLI_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set VULKAN_CLI_TEST_DATABASE_URL for alert snapshot integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	poolConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	poolConfig.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := fmt.Sprintf("alert_snapshot_test_%d", time.Now().UnixNano())
	client, err := NewClient(ctx, pool, &ClientConfig{Schema: schema, DisableManager: true, AllowDestroy: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := pool.Exec(cleanupCtx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	config := &SystemConfig{PartitionCountAlert: &PartitionCountAlertConfig{Threshold: 100}}
	if err := client.System().Register(ctx, config); err != nil {
		t.Fatal(err)
	}
	orders := client.Topic[RawPayload]("orders")
	if _, err := orders.Register(ctx, nil); err != nil {
		t.Fatal(err)
	}

	checks := []*AlertHandle{
		orders.Alerts().PartitionCount(),
		orders.Alerts().CompactionReadCost(),
		orders.Alerts().WorkerLiveness(),
		client.System().Alerts().MetricsCollectorProgress(),
	}
	checkReadOnly := func(wanted []AlertEvaluationState) {
		t.Helper()
		if _, err := pool.Exec(ctx, "SET default_transaction_read_only = on"); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if _, err := pool.Exec(ctx, "SET default_transaction_read_only = off"); err != nil {
				t.Error(err)
			}
		}()
		for index, handle := range checks {
			snapshot, err := handle.Snapshot(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if snapshot.State != wanted[index] || snapshot.EvaluatedAt.IsZero() || snapshot.PendingDuration != 2*time.Minute {
				t.Fatalf("%s snapshot = %+v", handle.name, snapshot)
			}
			if (snapshot.Reason != "") != (snapshot.State == AlertEvaluationStateInsufficientEvidence) {
				t.Fatalf("%s reason does not match state", handle.name)
			}
		}
	}
	checkReadOnly([]AlertEvaluationState{
		AlertEvaluationStateInsufficientEvidence, AlertEvaluationStateInsufficientEvidence,
		AlertEvaluationStateInsufficientEvidence, AlertEvaluationStateInsufficientEvidence,
	})

	producer, err := client.Topic[Measurement](metrics.MetricsTopicName).Producer().Register(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now()
	partitions, err := metrics.NewBuiltInMeasurement(metrics.MetricTopicPartitions, 105, map[string]string{"topic": "orders"}, at)
	if err != nil {
		t.Fatal(err)
	}
	partitions.Metadata = []byte(`{"compaction_status":"uncompacted"}`)
	workers, err := metrics.NewBuiltInMeasurement(metrics.MetricTopicUnclaimedWorkers, 0, map[string]string{"topic": "orders"}, at)
	if err != nil {
		t.Fatal(err)
	}
	workers.Metadata = []byte(`{"workers":[]}`)
	completion, err := metrics.NewBuiltInMeasurement(metrics.MetricCollectorCompletedTimestamp, float64(at.Unix()), nil, at)
	if err != nil {
		t.Fatal(err)
	}
	for _, measurement := range []*Measurement{partitions, workers, completion} {
		if _, err := producer.Produce(ctx, measurement, &ProduceOptions{
			RoutingKey: measurement.Name,
			MessageKey: metrics.MeasurementKey(measurement.Name, measurement.Attributes),
			Compaction: &CompactionOptions{Enable: true},
		}); err != nil {
			t.Fatal(err)
		}
	}
	checkReadOnly([]AlertEvaluationState{
		AlertEvaluationStatePending, AlertEvaluationStateHealthy,
		AlertEvaluationStateHealthy, AlertEvaluationStateHealthy,
	})
	latest, err := orders.Alerts().PartitionCount().Latest(ctx)
	if err != nil || latest != nil {
		t.Fatalf("snapshot recorded an alert: %+v, %v", latest, err)
	}

	config.PartitionCountAlert.DisablePending = true
	if err := client.System().Register(ctx, config); err != nil {
		t.Fatal(err)
	}
	if err := client.admin.SuspendSchedule(ctx, "alert.partition_count"); err != nil {
		t.Fatal(err)
	}
	checkReadOnly([]AlertEvaluationState{
		AlertEvaluationStateActive, AlertEvaluationStateHealthy,
		AlertEvaluationStateHealthy, AlertEvaluationStateHealthy,
	})
	snapshot, err := orders.Alerts().PartitionCount().Snapshot(ctx)
	if err != nil || snapshot.State != AlertEvaluationStateActive || !snapshot.DisablePending || snapshot.ObservedDuration != 0 {
		t.Fatalf("current suspended declaration snapshot = %+v, %v", snapshot, err)
	}
	for _, handle := range []*AlertHandle{orders.Alerts().Alert("custom"), client.System().Alerts().Alert("partition_count")} {
		if _, err := handle.Snapshot(ctx); err == nil {
			t.Fatal("unsupported alert snapshot succeeded")
		}
	}
	if _, err := client.Topic[RawPayload]("missing").Alerts().PartitionCount().Snapshot(ctx); !errors.Is(err, ErrTopicNotFound) {
		t.Fatalf("missing owner = %v", err)
	}
	for _, payload := range []string{`null`, `{"pending_duration":-1}`, `{"threshold":"wrong-type"}`} {
		if _, err := pool.Exec(ctx, "UPDATE "+schema+".schedule_config SET payload = $1::jsonb WHERE name = 'alert.partition_count'", payload); err != nil {
			t.Fatal(err)
		}
		if _, err := orders.Alerts().PartitionCount().Snapshot(ctx); err == nil {
			t.Fatal("malformed policy used a default")
		}
	}
	if err := client.admin.DestroySchedule(ctx, "alert.partition_count"); err != nil {
		t.Fatal(err)
	}
	if _, err := orders.Alerts().PartitionCount().Snapshot(ctx); !errors.Is(err, ErrScheduleNotFound) {
		t.Fatalf("missing schedule = %v", err)
	}
}
