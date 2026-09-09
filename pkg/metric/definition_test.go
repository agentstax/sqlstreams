package metric

import (
	"testing"

	"github.com/agentstax/sqlstreams/pkg/common/diagnostic"
)

func TestDefinitionsCarriesRegisteredMetadata(t *testing.T) {
	definitions := Definitions()
	backlog := definitionByName(t, definitions, MetricCursorBacklog.Name)

	if backlog.Code != "SQL0083" || backlog.Kind != MetricKindGauge || backlog.Unit != MetricUnitCount("message") {
		t.Fatalf("definition = %+v", backlog)
	}
	if backlog.Scope != diagnostic.MetricScopeConsumerGroup {
		t.Fatalf("scope = %q", backlog.Scope)
	}
	if len(backlog.AttributeKeys) != 2 || backlog.AttributeKeys[0] != "stream" || backlog.AttributeKeys[1] != "group" {
		t.Fatalf("attribute keys = %v", backlog.AttributeKeys)
	}
}

func TestDefinitionsFiltersScopes(t *testing.T) {
	definitions := Definitions(diagnostic.MetricScopeStream, diagnostic.MetricScopeConsumerSession)
	for _, definition := range definitions {
		if definition.Scope != diagnostic.MetricScopeStream && definition.Scope != diagnostic.MetricScopeConsumerSession {
			t.Fatalf("definition %s has unrequested scope %q", definition.Name, definition.Scope)
		}
	}
	if len(definitions) != 13 {
		t.Fatalf("got %d definitions, want 13", len(definitions))
	}
}

func TestDefinitionsSeparatesExporterHealth(t *testing.T) {
	definitions := Definitions(diagnostic.MetricScopeExporter)
	if len(definitions) != 2 {
		t.Fatalf("got %d exporter definitions, want 2", len(definitions))
	}
	for _, metric := range []*diagnostic.DiagnosticMetric{MetricOTelSourceReadSuccess, MetricOTelMeasurementsRejected} {
		definition := definitionByName(t, definitions, metric.Name)
		if definition.Scope != diagnostic.MetricScopeExporter || len(definition.AttributeKeys) != 0 {
			t.Fatalf("exporter definition = %+v", definition)
		}
		definitionByName(t, Definitions(), metric.Name)
	}
}

func TestDefinitionsReturnsDefensiveAttributeKeys(t *testing.T) {
	first := Definitions(diagnostic.MetricScopeConsumerGroup)
	backlog := definitionByName(t, first, MetricCursorBacklog.Name)
	backlog.AttributeKeys[0] = "changed"

	second := Definitions(diagnostic.MetricScopeConsumerGroup)
	backlog = definitionByName(t, second, MetricCursorBacklog.Name)
	if backlog.AttributeKeys[0] != "stream" {
		t.Fatalf("definition mutated catalog state: %v", backlog.AttributeKeys)
	}
}

func TestDefinitionsOrderedByCode(t *testing.T) {
	definitions := Definitions()
	for i := 1; i < len(definitions); i++ {
		if definitions[i-1].Code >= definitions[i].Code {
			t.Fatalf("codes out of order: %s before %s", definitions[i-1].Code, definitions[i].Code)
		}
	}
}

func definitionByName(t *testing.T, definitions []MetricDefinition, name string) MetricDefinition {
	t.Helper()
	for _, definition := range definitions {
		if definition.Name == name {
			return definition
		}
	}
	t.Fatalf("definition %q not found", name)
	return MetricDefinition{}
}
