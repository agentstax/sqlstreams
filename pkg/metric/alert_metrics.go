package metric

import "github.com/agentstax/sqlstreams/pkg/common/diagnostic"

var MetricActiveAlerts = diagnostic.NewDiagnosticMetric(
	"SS0073",
	"sqlstreams.alert.state.active_alerts",
	string(MetricKindGauge),
	string(MetricUnitCount("alert")),
	"retained alert heads whose status is active",
	diagnostic.MetricScopeSystem,
)

var MetricResolvedAlerts = diagnostic.NewDiagnosticMetric(
	"SS0074",
	"sqlstreams.alert.state.resolved_alerts",
	string(MetricKindGauge),
	string(MetricUnitCount("alert")),
	"retained alert heads whose status is resolved",
	diagnostic.MetricScopeSystem,
)

var MetricCheckStreamsEvaluated = diagnostic.NewDiagnosticMetric(
	"SS0075",
	"sqlstreams.alert.check.streams_evaluated",
	string(MetricKindGauge),
	string(MetricUnitCount("stream")),
	"streams the alert check evaluated",
	diagnostic.MetricScopeSystem,
	"alert",
)

var MetricCheckStreamsFailed = diagnostic.NewDiagnosticMetric(
	"SS0076",
	"sqlstreams.alert.check.streams_failed",
	string(MetricKindGauge),
	string(MetricUnitCount("stream")),
	"streams whose alert evaluation or result production did not complete",
	diagnostic.MetricScopeSystem,
	"alert",
)

var MetricCheckPublishedAlerts = diagnostic.NewDiagnosticMetric(
	"SS0077",
	"sqlstreams.alert.check.published_alerts",
	string(MetricKindGauge),
	string(MetricUnitCount("alert")),
	"alerts the check changed to active",
	diagnostic.MetricScopeSystem,
	"alert",
)

var MetricCheckResolvedAlerts = diagnostic.NewDiagnosticMetric(
	"SS0078",
	"sqlstreams.alert.check.resolved_alerts",
	string(MetricKindGauge),
	string(MetricUnitCount("alert")),
	"alerts the check changed to resolved",
	diagnostic.MetricScopeSystem,
	"alert",
)
