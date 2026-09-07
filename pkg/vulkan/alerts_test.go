package vulkan

import (
	"testing"

	"github.com/agentstax/vulkan/pkg/alert"
	"github.com/agentstax/vulkan/pkg/common/diagnostic"
)

func TestAlertSelectorsCoverResourceScopedCatalog(t *testing.T) {
	client := &Client{}
	topicAlerts := client.Topic[RawPayload]("orders").Alerts()

	selectors := []struct {
		name     string
		handle   *AlertHandle
		declared *diagnostic.DiagnosticAlert
	}{
		{"PartitionCount", topicAlerts.PartitionCount(), alert.AlertPartitionCount},
		{"CompactionReadCost", topicAlerts.CompactionReadCost(), alert.AlertCompactionReadCost},
		{"WorkerLiveness", topicAlerts.WorkerLiveness(), alert.AlertWorkerLiveness},
		{"MetricsCollectorProgress", client.System().Alerts().MetricsCollectorProgress(), alert.AlertMetricsCollectorProgress},
	}

	seen := make(map[*diagnostic.DiagnosticAlert]int, len(selectors))
	for _, selector := range selectors {
		if selector.handle.name != selector.declared.Name {
			t.Errorf("%s resolved %q, want %q", selector.name, selector.handle.name, selector.declared.Name)
		}
		topicName := "orders"
		if selector.declared.Scope == diagnostic.MetricScopeSystem {
			topicName = ""
		}
		if selector.handle.topicName != topicName || selector.handle.groupName != "" {
			t.Errorf("%s bound owner %q/%q, want topic %q", selector.name, selector.handle.topicName, selector.handle.groupName, topicName)
		}
		seen[selector.declared]++
	}

	definitions := alert.Definitions(
		diagnostic.MetricScopeSystem,
		diagnostic.MetricScopeTopic,
		diagnostic.MetricScopeConsumerGroup,
	)
	if len(selectors) != len(definitions) {
		t.Fatalf("%d selectors cover %d definitions", len(selectors), len(definitions))
	}
	for _, definition := range definitions {
		declared, found := diagnostic.GetAlert(definition.Name)
		if !found {
			t.Errorf("definition %q is not registered", definition.Name)
			continue
		}
		if seen[declared] != 1 {
			t.Errorf("definition %q appears through %d selectors, want 1", definition.Name, seen[declared])
		}
	}
}

func TestAlertHandleConstructorsPerformNoIO(t *testing.T) {
	client := &Client{}

	if len(client.System().Alerts().Definitions()) != len(alert.Definitions()) {
		t.Fatal("system definitions do not expose the complete catalog")
	}
	if len(client.Topic[RawPayload]("orders").Alerts().Definitions()) != 3 {
		t.Fatal("topic definitions do not expose the topic catalog")
	}
	if len(client.Topic[RawPayload]("orders").Consumer("billing").Alerts().Definitions()) != 0 {
		t.Fatal("group definitions expose a definition no check owns")
	}

	system := client.System().Alerts().Alert("disk_pressure")
	if system.name != "disk_pressure" || system.topicName != "" || system.groupName != "" {
		t.Fatalf("system alert bound %+v", system)
	}
	group := client.Topic[RawPayload]("orders").Consumer("billing").Alerts().Alert("disk_pressure")
	if group.topicName != "orders" || group.groupName != "billing" {
		t.Fatalf("group alert bound %+v", group)
	}
}
