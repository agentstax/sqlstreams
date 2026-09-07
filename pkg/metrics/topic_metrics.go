package metrics

import "github.com/agentstax/vulkan/pkg/common/diagnostic"

var MetricTopicPartitions = diagnostic.NewDiagnosticMetric(
	"VK0098",
	"vulkan.topic.state.partitions",
	string(MetricKindGauge),
	string(MetricUnitCount("partition")),
	"partitions on the topic's message log",
	diagnostic.MetricScopeTopic,
	"topic",
)

var MetricTopicCompacted = diagnostic.NewDiagnosticMetric(
	"VK0079",
	"vulkan.topic.state.compacted",
	string(MetricKindGauge),
	"",
	"1 once the topic has received a keyed message, otherwise 0",
	diagnostic.MetricScopeTopic,
	"topic",
)
