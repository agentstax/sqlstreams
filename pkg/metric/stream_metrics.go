package metric

import "github.com/allegedlyreliable/sqlstreams/pkg/common/diagnostic"

var MetricStreamUnclaimedWorkers = diagnostic.NewDiagnosticMetric(
	"SQL0099",
	"sqlstreams.stream.workers.unclaimed",
	string(MetricKindGauge),
	string(MetricUnitCount("worker")),
	"unclaimed workers owned by the stream or its consumer groups",
	diagnostic.MetricScopeStream,
	"stream",
)

var MetricStreamPartitions = diagnostic.NewDiagnosticMetric(
	"SQL0098",
	"sqlstreams.stream.state.partitions",
	string(MetricKindGauge),
	string(MetricUnitCount("partition")),
	"partitions on the stream's message log",
	diagnostic.MetricScopeStream,
	"stream",
)

var MetricStreamCompacted = diagnostic.NewDiagnosticMetric(
	"SQL0079",
	"sqlstreams.stream.state.compacted",
	string(MetricKindGauge),
	"",
	"1 once the stream has received a keyed message, otherwise 0",
	diagnostic.MetricScopeStream,
	"stream",
)
