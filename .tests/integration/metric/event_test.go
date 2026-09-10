package metric

import (
	"slices"
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/pkg/metric"
	"github.com/agentstax/sqlstreams/pkg/metric/controller/datastore"
)

// behavior: event timestamps read the metrics stream's payload documents
// under one routing key and event type, one row per distinct (message,
// attempt) at the earliest time seen, leaving other keys and types out.
func TestEventTimestampsGroupTheEarliestTimePerMessageAttempt(t *testing.T) {
	// setup
	metrics, orders := newMetricDatastore(t)
	ctx := t.Context()
	metricsStream := registerMetricsStream(t, metrics, orders)
	first := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	insertMetricEvent(t, metrics, metricsStream, "orders:processor", metric.EventAbandoned, 7, 1, first.Add(2*time.Minute))
	insertMetricEvent(t, metrics, metricsStream, "orders:processor", metric.EventAbandoned, 7, 1, first)
	insertMetricEvent(t, metrics, metricsStream, "orders:processor", metric.EventAbandoned, 7, 2, first.Add(time.Minute))
	insertMetricEvent(t, metrics, metricsStream, "orders:processor", metric.EventCleared, 7, 1, first.Add(3*time.Minute))
	insertMetricEvent(t, metrics, metricsStream, "orders:auditor", metric.EventAbandoned, 9, 1, first)

	// test
	abandoned, err := metrics.EventTimestamps(ctx, "orders:processor", metric.EventAbandoned)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if len(abandoned) != 2 {
		t.Fatalf("EventTimestamps(orders:processor, abandoned) = %+v, want two rows", abandoned)
	}
	slices.SortFunc(abandoned, func(a datastore.EventTimestampRow, b datastore.EventTimestampRow) int { return a.Attempt - b.Attempt })
	if abandoned[0].MessageId != 7 || abandoned[0].Attempt != 1 || !abandoned[0].At.Equal(first) {
		t.Errorf("EventTimestamps(orders:processor, abandoned)[attempt 1] = %+v, want message 7 at the earliest time 12:00", abandoned[0])
	}
	if abandoned[1].MessageId != 7 || abandoned[1].Attempt != 2 || !abandoned[1].At.Equal(first.Add(time.Minute)) {
		t.Errorf("EventTimestamps(orders:processor, abandoned)[attempt 2] = %+v, want message 7 at 12:01", abandoned[1])
	}
}
