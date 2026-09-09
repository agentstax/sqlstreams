package metric

import "github.com/agentstax/vulkan/pkg/common/diagnostic"

var MetricCollectorCompletedTimestamp = diagnostic.NewDiagnosticMetric(
	"VK0100",
	"vulkan.metrics.collector.completed_timestamp",
	string(MetricKindGauge),
	"s",
	"Unix timestamp of the last completed metrics collection pass",
	diagnostic.MetricScopeSystem,
)
