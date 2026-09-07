package alert

import (
	"github.com/agentstax/vulkan/pkg/common/diagnostic"
)

// Built-in alert names are wire values and the message key's first segment.
var AlertPartitionCount = diagnostic.NewDiagnosticAlert("VK0094",
	"partition_count",
	"a topic's message log holds enough partitions that dropping the topic approaches the lock-table ceiling",
	diagnostic.MetricScopeTopic, string(AlertSeverityWarn))

var AlertCompactionReadCost = diagnostic.NewDiagnosticAlert("VK0095",
	"compaction_read_cost",
	"a compacted topic holds enough partitions that replaying a never-superseded key is a long scan",
	diagnostic.MetricScopeTopic, string(AlertSeverityWarn))

var AlertWorkerLiveness = diagnostic.NewDiagnosticAlert("VK0096",
	"worker_liveness",
	"a topic's worker rows have no live instance, so nothing runs its upkeep",
	diagnostic.MetricScopeTopic, string(AlertSeverityWarn))

var AlertMetricsCollectorProgress = diagnostic.NewDiagnosticAlert("VK0101",
	"metrics_collector_progress",
	"metrics collection has not completed within the allowed age while manager leases remain continuous",
	diagnostic.MetricScopeSystem, string(AlertSeverityWarn))
