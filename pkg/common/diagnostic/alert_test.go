package diagnostic

import "testing"

var alertTestPartitionCount = NewDiagnosticAlert(
	"SS9930",
	"test_partition_count",
	"a test stream holds more partitions than the threshold",
	MetricScopeStream,
	"warn",
)

func TestAlertCarriesMetadata(t *testing.T) {
	if alertTestPartitionCount.Scope != MetricScopeStream || alertTestPartitionCount.Severity != "warn" {
		t.Fatalf("declaration = %+v", alertTestPartitionCount)
	}
	if alertTestPartitionCount.GetKind() != DiagnosticKindAlert {
		t.Fatalf("kind = %q", alertTestPartitionCount.GetKind())
	}
}

func TestGetAlertResolvesByName(t *testing.T) {
	declared, ok := GetAlert("test_partition_count")
	if !ok || declared != alertTestPartitionCount {
		t.Fatalf("GetAlert = %+v, %v", declared, ok)
	}
	if _, ok := GetAlert("test_unregistered"); ok {
		t.Fatal("GetAlert resolved a name nothing declared")
	}
}

func TestNewAlertRejectsInvalidMetadata(t *testing.T) {
	tests := []struct {
		name        string
		code        string
		alertName   string
		description string
		scope       MetricScope
		severity    string
	}{
		{name: "empty name", code: "SS9931", description: "test condition", scope: MetricScopeStream, severity: "warn"},
		{name: "empty description", code: "SS9932", alertName: "test_empty_description", scope: MetricScopeStream, severity: "warn"},
		{name: "empty scope", code: "SS9933", alertName: "test_empty_scope", description: "test condition", severity: "warn"},
		{name: "unknown scope", code: "SS9934", alertName: "test_unknown_scope", description: "test condition", scope: MetricScope("worker"), severity: "warn"},
		{name: "session scope", code: "SS9935", alertName: "test_session_scope", description: "test condition", scope: MetricScopeConsumerSession, severity: "warn"},
		{name: "exporter scope", code: "SS9938", alertName: "test_exporter_scope", description: "test condition", scope: MetricScopeExporter, severity: "warn"},
		{name: "empty severity", code: "SS9936", alertName: "test_empty_severity", description: "test condition", scope: MetricScopeStream},
		{name: "duplicate name", code: "SS9937", alertName: "test_partition_count", description: "test condition", scope: MetricScopeStream, severity: "warn"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expectPanic(t, func() {
				NewDiagnosticAlert(test.code, test.alertName, test.description, test.scope, test.severity)
			})
		})
	}
}

func TestAlertsListsOrderedByCode(t *testing.T) {
	listed := Alerts()
	for i := 1; i < len(listed); i++ {
		if listed[i-1].Code >= listed[i].Code {
			t.Fatalf("codes out of order: %s before %s", listed[i-1].Code, listed[i].Code)
		}
	}
}
