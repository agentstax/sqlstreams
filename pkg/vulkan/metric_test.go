package vulkan

import (
	"testing"

	"github.com/agentstax/vulkan/pkg/common/diagnostic"
	"github.com/agentstax/vulkan/pkg/metric"
)

func TestMetricSelectorsCoverResourceScopedCatalog(t *testing.T) {
	client := &Client{}
	systemMetrics := client.System().Metrics()
	topicMetrics := client.Topic[RawPayload]("orders").Metrics()
	groupMetrics := client.Topic[RawPayload]("orders").Consumer("billing").Metrics()

	selectors := []struct {
		name       string
		handle     *MetricHandle
		declared   *diagnostic.DiagnosticMetric
		attributes map[string]string
	}{
		{"CollectorCompletedTimestamp", systemMetrics.CollectorCompletedTimestamp(), metric.MetricCollectorCompletedTimestamp, nil},
		{"UnclaimedWorkers", systemMetrics.UnclaimedWorkers(), metric.MetricUnclaimedWorkers, nil},
		{"OldestUnclaimedAge", systemMetrics.OldestUnclaimedAge(), metric.MetricOldestUnclaimedAge, nil},
		{"FailingWorkers", systemMetrics.FailingWorkers(), metric.MetricFailingWorkers, nil},
		{"OverdueSchedules", systemMetrics.OverdueSchedules(), metric.MetricOverdueSchedules, nil},
		{"OldestDueAge", systemMetrics.OldestDueAge(), metric.MetricOldestDueAge, nil},
		{"SuspendedSchedules", systemMetrics.SuspendedSchedules(), metric.MetricSuspendedSchedules, nil},
		{"ActiveAlerts", systemMetrics.ActiveAlerts(), metric.MetricActiveAlerts, nil},
		{"ResolvedAlerts", systemMetrics.ResolvedAlerts(), metric.MetricResolvedAlerts, nil},
		{"CheckTopicsEvaluated", systemMetrics.CheckTopicsEvaluated("partition_count"), metric.MetricCheckTopicsEvaluated, map[string]string{"alert": "partition_count"}},
		{"CheckTopicsFailed", systemMetrics.CheckTopicsFailed("partition_count"), metric.MetricCheckTopicsFailed, map[string]string{"alert": "partition_count"}},
		{"CheckPublishedAlerts", systemMetrics.CheckPublishedAlerts("partition_count"), metric.MetricCheckPublishedAlerts, map[string]string{"alert": "partition_count"}},
		{"CheckResolvedAlerts", systemMetrics.CheckResolvedAlerts("partition_count"), metric.MetricCheckResolvedAlerts, map[string]string{"alert": "partition_count"}},
		{"Compacted", topicMetrics.Compacted(), metric.MetricTopicCompacted, map[string]string{"topic": "orders"}},
		{"Partitions", topicMetrics.Partitions(), metric.MetricTopicPartitions, map[string]string{"topic": "orders"}},
		{"TopicUnclaimedWorkers", topicMetrics.UnclaimedWorkers(), metric.MetricTopicUnclaimedWorkers, map[string]string{"topic": "orders"}},
		{"CursorHead", groupMetrics.CursorHead(), metric.MetricCursorHead, map[string]string{"topic": "orders", "group": "billing"}},
		{"CursorClaimed", groupMetrics.CursorClaimed(), metric.MetricCursorClaimed, map[string]string{"topic": "orders", "group": "billing"}},
		{"CursorCommitted", groupMetrics.CursorCommitted(), metric.MetricCursorCommitted, map[string]string{"topic": "orders", "group": "billing"}},
		{"CursorBacklog", groupMetrics.CursorBacklog(), metric.MetricCursorBacklog, map[string]string{"topic": "orders", "group": "billing"}},
		{"CursorInflight", groupMetrics.CursorInflight(), metric.MetricCursorInflight, map[string]string{"topic": "orders", "group": "billing"}},
		{"ReadyExceptions", groupMetrics.ReadyExceptions(), metric.MetricReadyExceptions, map[string]string{"topic": "orders", "group": "billing"}},
		{"InflightExceptions", groupMetrics.InflightExceptions(), metric.MetricInflightExceptions, map[string]string{"topic": "orders", "group": "billing"}},
		{"DeferredExceptions", groupMetrics.DeferredExceptions(), metric.MetricDeferredExceptions, map[string]string{"topic": "orders", "group": "billing"}},
		{"DeadExceptions", groupMetrics.DeadExceptions(), metric.MetricDeadExceptions, map[string]string{"topic": "orders", "group": "billing"}},
		{"OldestUnresolvedAge", groupMetrics.OldestUnresolvedAge(), metric.MetricOldestUnresolvedAge, map[string]string{"topic": "orders", "group": "billing"}},
		{"OpenLeases", groupMetrics.OpenLeases(), metric.MetricOpenLeases, map[string]string{"topic": "orders", "group": "billing"}},
		{"AbandonedRoutinesOutstanding", groupMetrics.AbandonedRoutinesOutstanding(), metric.MetricAbandonedOutstanding, map[string]string{"topic": "orders", "group": "billing"}},
		{"AbandonedRoutinesTotal", groupMetrics.AbandonedRoutinesTotal(), metric.MetricAbandonedTotal, map[string]string{"topic": "orders", "group": "billing"}},
		{"AbandonedRoutinesSelfClearLatencyAverage", groupMetrics.AbandonedRoutinesSelfClearLatencyAverage(), metric.MetricAbandonedSelfClearLatencyAvg, map[string]string{"topic": "orders", "group": "billing"}},
	}

	seen := make(map[*diagnostic.DiagnosticMetric]int, len(selectors))
	for _, selector := range selectors {
		if selector.handle.declared != selector.declared {
			t.Errorf("%s resolved %p, want %p", selector.name, selector.handle.declared, selector.declared)
		}
		wantedMessageKey := metric.MeasurementKey(selector.declared.Name, selector.attributes)
		if selector.handle.messageKey != wantedMessageKey {
			t.Errorf("%s message key = %q, want %q", selector.name, selector.handle.messageKey, wantedMessageKey)
		}
		seen[selector.handle.declared]++
	}

	definitions := metric.Definitions(
		diagnostic.MetricScopeSystem,
		diagnostic.MetricScopeTopic,
		diagnostic.MetricScopeConsumerGroup,
	)
	if len(selectors) != len(definitions) {
		t.Fatalf("%d selectors cover %d definitions", len(selectors), len(definitions))
	}
	for _, definition := range definitions {
		declared, found := diagnostic.GetMetric(definition.Name)
		if !found {
			t.Errorf("definition %q is not registered", definition.Name)
			continue
		}
		if seen[declared] != 1 {
			t.Errorf("definition %q appears through %d selectors, want 1", definition.Name, seen[declared])
		}
	}
}

func TestMetricHandleConstructorsPerformNoIO(t *testing.T) {
	client := &Client{}

	systemMetrics := client.System().Metrics()
	if len(systemMetrics.Definitions()) != len(metric.Definitions()) {
		t.Fatal("system definitions do not expose the complete catalog")
	}
	if len(client.Topic[RawPayload]("orders").Metrics().Definitions()) != 3 {
		t.Fatal("topic definitions do not expose the topic catalog")
	}
	if len(client.Topic[RawPayload]("orders").Consumer("billing").Metrics().Definitions()) != 14 {
		t.Fatal("group definitions do not expose the consumer-group catalog")
	}

	known := systemMetrics.Metric(metric.MetricCursorBacklog.Name, map[string]string{"topic": "orders", "group": "billing"})
	if known.declared != metric.MetricCursorBacklog {
		t.Fatal("arbitrary selector did not bind its registered definition")
	}
	custom := systemMetrics.Metric("checkout.request.duration", map[string]string{"region": "us-east-1"})
	if custom.declared != nil {
		t.Fatal("user metric unexpectedly bound a Vulkan definition")
	}
}

func TestUnwrapMessagesRemovesStorageEnvelopes(t *testing.T) {
	first := &Measurement{Name: "first"}
	second := &Measurement{Name: "second"}
	stored := []*StoredMessage[Measurement]{
		{Id: 41, Message: first, MessageKey: "first"},
		{Id: 73, Message: second, MessageKey: "second"},
	}

	measurements := unwrapMessages(stored)
	if len(measurements) != 2 || measurements[0] != first || measurements[1] != second {
		t.Fatalf("measurements = %+v", measurements)
	}
	empty := unwrapMessages[Measurement](nil)
	if empty == nil || len(empty) != 0 {
		t.Fatalf("empty measurements = %#v", empty)
	}
}
