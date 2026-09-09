package sqlstreams

import (
	"testing"

	"github.com/agentstax/sqlstreams/pkg/alert"
	"github.com/agentstax/sqlstreams/pkg/common/diagnostic"
)

func TestAlertSelectorsCoverResourceScopedCatalog(t *testing.T) {
	client := &Client{}
	streamAlerts := client.Stream[RawPayload]("orders").Alerts()

	selectors := []struct {
		name     string
		handle   *AlertHandle
		declared *diagnostic.DiagnosticAlert
	}{
		{"PartitionCount", streamAlerts.PartitionCount(), alert.AlertPartitionCount},
		{"CompactionReadCost", streamAlerts.CompactionReadCost(), alert.AlertCompactionReadCost},
		{"WorkerLiveness", streamAlerts.WorkerLiveness(), alert.AlertWorkerLiveness},
		{"MetricCollectorProgress", client.System().Alerts().MetricCollectorProgress(), alert.AlertMetricsCollectorProgress},
	}

	seen := make(map[*diagnostic.DiagnosticAlert]int, len(selectors))
	for _, selector := range selectors {
		if selector.handle.name != selector.declared.Name {
			t.Errorf("%s resolved %q, want %q", selector.name, selector.handle.name, selector.declared.Name)
		}
		streamName := "orders"
		if selector.declared.Scope == diagnostic.MetricScopeSystem {
			streamName = ""
		}
		if selector.handle.streamName != streamName || selector.handle.groupName != "" {
			t.Errorf("%s bound owner %q/%q, want stream %q", selector.name, selector.handle.streamName, selector.handle.groupName, streamName)
		}
		seen[selector.declared]++
	}

	definitions := alert.Definitions(
		diagnostic.MetricScopeSystem,
		diagnostic.MetricScopeStream,
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
	if len(client.Stream[RawPayload]("orders").Alerts().Definitions()) != 3 {
		t.Fatal("stream definitions do not expose the stream catalog")
	}
	if len(client.Stream[RawPayload]("orders").Consumer("billing").Alerts().Definitions()) != 0 {
		t.Fatal("group definitions expose a definition no check owns")
	}

	system := client.System().Alerts().Alert("disk_pressure")
	if system.name != "disk_pressure" || system.streamName != "" || system.groupName != "" {
		t.Fatalf("system alert bound %+v", system)
	}
	group := client.Stream[RawPayload]("orders").Consumer("billing").Alerts().Alert("disk_pressure")
	if group.streamName != "orders" || group.groupName != "billing" {
		t.Fatalf("group alert bound %+v", group)
	}
}
