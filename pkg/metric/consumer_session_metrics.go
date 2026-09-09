package metric

import "github.com/agentstax/sqlstreams/pkg/common/diagnostic"

// Consumer-session flows are per-instance monotonic totals, one series per
// session. They report what one instance did.
var MetricSessionClaimed = diagnostic.NewDiagnosticMetric(
	"SS0042",
	"sqlstreams.consumer.session.claimed",
	string(MetricKindCounter),
	string(MetricUnitCount("message")),
	"messages this instance claimed -- cursor ranges and exception retries together",
	diagnostic.MetricScopeConsumerSession,
	"stream",
	"group",
	"version",
	"session",
)

var MetricSessionSuccess = diagnostic.NewDiagnosticMetric(
	"SS0043",
	"sqlstreams.consumer.session.success",
	string(MetricKindCounter),
	string(MetricUnitCount("message")),
	"consumerFunc runs that completed cleanly, counted at resolution",
	diagnostic.MetricScopeConsumerSession,
	"stream",
	"group",
	"version",
	"session",
)

var MetricSessionSuperseded = diagnostic.NewDiagnosticMetric(
	"SS0044",
	"sqlstreams.consumer.session.superseded",
	string(MetricKindCounter),
	string(MetricUnitCount("message")),
	"messages resolved without running -- a newer version of their compacted message key had already arrived",
	diagnostic.MetricScopeConsumerSession,
	"stream",
	"group",
	"version",
	"session",
)

var MetricSessionReady = diagnostic.NewDiagnosticMetric(
	"SS0045",
	"sqlstreams.consumer.session.ready",
	string(MetricKindCounter),
	string(MetricUnitCount("message")),
	"delivery rows written 'ready' -- each will be retried once its backoff passes",
	diagnostic.MetricScopeConsumerSession,
	"stream",
	"group",
	"version",
	"session",
)

var MetricSessionDeferred = diagnostic.NewDiagnosticMetric(
	"SS0046",
	"sqlstreams.consumer.session.deferred",
	string(MetricKindCounter),
	string(MetricUnitCount("message")),
	"delivery rows written 'deferred' -- another delivery held their message key",
	diagnostic.MetricScopeConsumerSession,
	"stream",
	"group",
	"version",
	"session",
)

var MetricSessionDead = diagnostic.NewDiagnosticMetric(
	"SS0047",
	"sqlstreams.consumer.session.dead",
	string(MetricKindCounter),
	string(MetricUnitCount("message")),
	"delivery rows written 'dead' -- exhausted retries, unrecoverable payloads, and the kill backstop",
	diagnostic.MetricScopeConsumerSession,
	"stream",
	"group",
	"version",
	"session",
)

var MetricSessionReclaimed = diagnostic.NewDiagnosticMetric(
	"SS0048",
	"sqlstreams.consumer.session.reclaimed",
	string(MetricKindCounter),
	string(MetricUnitCount("lease")),
	"leases taken over from expired workers -- another instance died or stalled mid-range",
	diagnostic.MetricScopeConsumerSession,
	"stream",
	"group",
	"version",
	"session",
)

var MetricSessionQuarantined = diagnostic.NewDiagnosticMetric(
	"SS0049",
	"sqlstreams.consumer.session.quarantined",
	string(MetricKindCounter),
	string(MetricUnitCount("range")),
	"ranges past the reclaim cap, written out as independent 'ready' exceptions",
	diagnostic.MetricScopeConsumerSession,
	"stream",
	"group",
	"version",
	"session",
)

var MetricSessionAbandoned = diagnostic.NewDiagnosticMetric(
	"SS0050",
	"sqlstreams.consumer.session.abandoned",
	string(MetricKindCounter),
	string(MetricUnitCount("routine")),
	"consumerFunc goroutines written off past the hard timeout",
	diagnostic.MetricScopeConsumerSession,
	"stream",
	"group",
	"version",
	"session",
)

var MetricSessionLeaseLost = diagnostic.NewDiagnosticMetric(
	"SS0051",
	"sqlstreams.consumer.session.lease_lost",
	string(MetricKindCounter),
	string(MetricUnitCount("commit")),
	"commits rejected because another worker reclaimed the lease first -- that work was redone elsewhere",
	diagnostic.MetricScopeConsumerSession,
	"stream",
	"group",
	"version",
	"session",
)
