package diagnostic

import "testing"

var metricTestBacklog = NewDiagnosticMetric(
	"SS9920",
	"sqlstreams.test.cursor.backlog",
	"gauge",
	"{message}",
	"test messages beyond the committed cursor",
	MetricScopeConsumerGroup,
	"stream",
	"group",
)

func TestMetricCarriesMetadata(t *testing.T) {
	if metricTestBacklog.Scope != MetricScopeConsumerGroup {
		t.Fatalf("scope = %q", metricTestBacklog.Scope)
	}
	if len(metricTestBacklog.AttributeKeys) != 2 || metricTestBacklog.AttributeKeys[0] != "stream" || metricTestBacklog.AttributeKeys[1] != "group" {
		t.Fatalf("attribute keys = %v", metricTestBacklog.AttributeKeys)
	}
}

func TestNewMetricCopiesAttributeKeys(t *testing.T) {
	attributeKeys := []string{"stream"}
	declared := NewDiagnosticMetric(
		"SS9921",
		"sqlstreams.test.stream.depth",
		"gauge",
		"{message}",
		"test messages retained for a stream",
		MetricScopeStream,
		attributeKeys...,
	)
	attributeKeys[0] = "changed"

	if declared.AttributeKeys[0] != "stream" {
		t.Fatalf("constructor retained the caller's slice: %v", declared.AttributeKeys)
	}
}

func TestNewMetricRejectsInvalidMetadata(t *testing.T) {
	tests := []struct {
		name          string
		code          string
		metricName    string
		kind          string
		description   string
		scope         MetricScope
		attributeKeys []string
	}{
		{name: "empty name", code: "SS9922", kind: "gauge", description: "test depth", scope: MetricScopeSystem},
		{name: "empty kind", code: "SS9923", metricName: "sqlstreams.test.empty_kind", description: "test depth", scope: MetricScopeSystem},
		{name: "empty description", code: "SS9924", metricName: "sqlstreams.test.empty_description", kind: "gauge", scope: MetricScopeSystem},
		{name: "empty scope", code: "SS9925", metricName: "sqlstreams.test.empty_scope", kind: "gauge", description: "test depth"},
		{name: "unknown scope", code: "SS9926", metricName: "sqlstreams.test.unknown_scope", kind: "gauge", description: "test depth", scope: MetricScope("worker")},
		{name: "empty attribute key", code: "SS9927", metricName: "sqlstreams.test.empty_attribute", kind: "gauge", description: "test depth", scope: MetricScopeSystem, attributeKeys: []string{""}},
		{name: "duplicate attribute key", code: "SS9928", metricName: "sqlstreams.test.duplicate_attribute", kind: "gauge", description: "test depth", scope: MetricScopeSystem, attributeKeys: []string{"stream", "stream"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expectPanic(t, func() {
				NewDiagnosticMetric(test.code, test.metricName, test.kind, "", test.description, test.scope, test.attributeKeys...)
			})
		})
	}
}

func TestMetricsListsOrderedByCode(t *testing.T) {
	listed := Metrics()
	for i := 1; i < len(listed); i++ {
		if listed[i-1].Code >= listed[i].Code {
			t.Fatalf("codes out of order: %s before %s", listed[i-1].Code, listed[i].Code)
		}
	}
}
