package metric

import "github.com/agentstax/sqlstreams/pkg/common/diagnostic"

var MetricCollectorCompletedTimestamp = diagnostic.NewDiagnosticMetric(
	"SS0100",
	"sqlstreams.metrics.collector.completed_timestamp",
	string(MetricKindGauge),
	"s",
	"Unix timestamp of the last completed metrics collection pass",
	diagnostic.MetricScopeSystem,
)
