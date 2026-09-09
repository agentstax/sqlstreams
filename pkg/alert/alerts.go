package alert

import (
	"github.com/agentstax/sqlstreams/pkg/common/diagnostic"
)

// Built-in alert names are wire values and the message key's first segment.
var AlertPartitionCount = diagnostic.NewDiagnosticAlert("SQL0094",
	"partition_count",
	"a stream's message log holds enough partitions that dropping the stream approaches the lock-table ceiling",
	diagnostic.MetricScopeStream, string(AlertSeverityWarn))

var AlertCompactionReadCost = diagnostic.NewDiagnosticAlert("SQL0095",
	"compaction_read_cost",
	"a compacted stream holds enough partitions that replaying a never-superseded key is a long scan",
	diagnostic.MetricScopeStream, string(AlertSeverityWarn))

var AlertWorkerLiveness = diagnostic.NewDiagnosticAlert("SQL0096",
	"worker_liveness",
	"a stream's worker rows have no live instance, so nothing runs its upkeep",
	diagnostic.MetricScopeStream, string(AlertSeverityWarn))

var AlertMetricsCollectorProgress = diagnostic.NewDiagnosticAlert("SQL0101",
	"metrics_collector_progress",
	"metrics collection has not completed within the allowed age while manager leases remain continuous",
	diagnostic.MetricScopeSystem, string(AlertSeverityWarn))
