package metric

import "github.com/allegedlyreliable/sqlstreams/pkg/common/diagnostic"

var MetricCursorHead = diagnostic.NewDiagnosticMetric(
	"SQL0080",
	"sqlstreams.consumer.cursor.head",
	string(MetricKindGauge),
	string(MetricUnitCount("message")),
	"highest message id ever appended to the stream",
	diagnostic.MetricScopeConsumerGroup,
	"stream",
	"group",
)

var MetricCursorClaimed = diagnostic.NewDiagnosticMetric(
	"SQL0081",
	"sqlstreams.consumer.cursor.claimed",
	string(MetricKindGauge),
	string(MetricUnitCount("message")),
	"the consumer group's read frontier",
	diagnostic.MetricScopeConsumerGroup,
	"stream",
	"group",
)

var MetricCursorCommitted = diagnostic.NewDiagnosticMetric(
	"SQL0082",
	"sqlstreams.consumer.cursor.committed",
	string(MetricKindGauge),
	string(MetricUnitCount("message")),
	"the frontier at or below which every message is complete or dead",
	diagnostic.MetricScopeConsumerGroup,
	"stream",
	"group",
)

var MetricCursorBacklog = diagnostic.NewDiagnosticMetric(
	"SQL0083",
	"sqlstreams.consumer.cursor.backlog",
	string(MetricKindGauge),
	string(MetricUnitCount("message")),
	"messages beyond the consumer group's committed frontier",
	diagnostic.MetricScopeConsumerGroup,
	"stream",
	"group",
)

var MetricCursorInflight = diagnostic.NewDiagnosticMetric(
	"SQL0084",
	"sqlstreams.consumer.cursor.inflight",
	string(MetricKindGauge),
	string(MetricUnitCount("message")),
	"claimed messages beyond the consumer group's committed frontier",
	diagnostic.MetricScopeConsumerGroup,
	"stream",
	"group",
)

var MetricReadyExceptions = diagnostic.NewDiagnosticMetric(
	"SQL0085",
	"sqlstreams.consumer.exceptions.ready",
	string(MetricKindGauge),
	string(MetricUnitCount("exception")),
	"delivery rows waiting for their retry delay",
	diagnostic.MetricScopeConsumerGroup,
	"stream",
	"group",
)

var MetricInflightExceptions = diagnostic.NewDiagnosticMetric(
	"SQL0086",
	"sqlstreams.consumer.exceptions.inflight",
	string(MetricKindGauge),
	string(MetricUnitCount("exception")),
	"delivery rows held by a retry attempt",
	diagnostic.MetricScopeConsumerGroup,
	"stream",
	"group",
)

var MetricDeferredExceptions = diagnostic.NewDiagnosticMetric(
	"SQL0087",
	"sqlstreams.consumer.exceptions.deferred",
	string(MetricKindGauge),
	string(MetricUnitCount("exception")),
	"delivery rows waiting for their message key's lease to become free",
	diagnostic.MetricScopeConsumerGroup,
	"stream",
	"group",
)

var MetricDeadExceptions = diagnostic.NewDiagnosticMetric(
	"SQL0088",
	"sqlstreams.consumer.exceptions.dead",
	string(MetricKindGauge),
	string(MetricUnitCount("exception")),
	"dead-lettered delivery rows",
	diagnostic.MetricScopeConsumerGroup,
	"stream",
	"group",
)

var MetricOldestUnresolvedAge = diagnostic.NewDiagnosticMetric(
	"SQL0089",
	"sqlstreams.consumer.exceptions.oldest_unresolved_age",
	string(MetricKindGauge),
	string(MetricUnitMilliseconds),
	"age of the oldest ready, inflight, or deferred delivery row",
	diagnostic.MetricScopeConsumerGroup,
	"stream",
	"group",
)

var MetricOpenLeases = diagnostic.NewDiagnosticMetric(
	"SQL0090",
	"sqlstreams.consumer.open_leases",
	string(MetricKindGauge),
	string(MetricUnitCount("lease")),
	"open leases held by the consumer group",
	diagnostic.MetricScopeConsumerGroup,
	"stream",
	"group",
)

var MetricAbandonedOutstanding = diagnostic.NewDiagnosticMetric(
	"SQL0091",
	"sqlstreams.consumer.abandoned_routines.outstanding",
	string(MetricKindGauge),
	string(MetricUnitCount("routine")),
	"abandoned routines with no matching cleared record",
	diagnostic.MetricScopeConsumerGroup,
	"stream",
	"group",
)

var MetricAbandonedTotal = diagnostic.NewDiagnosticMetric(
	"SQL0092",
	"sqlstreams.consumer.abandoned_routines.total",
	string(MetricKindGauge),
	string(MetricUnitCount("routine")),
	"distinct abandoned routines retained on the metrics stream",
	diagnostic.MetricScopeConsumerGroup,
	"stream",
	"group",
)

var MetricAbandonedSelfClearLatencyAvg = diagnostic.NewDiagnosticMetric(
	"SQL0093",
	"sqlstreams.consumer.abandoned_routines.self_clear_latency_avg",
	string(MetricKindGauge),
	string(MetricUnitMilliseconds),
	"mean time between an abandoned routine and its matching cleared record",
	diagnostic.MetricScopeConsumerGroup,
	"stream",
	"group",
)
