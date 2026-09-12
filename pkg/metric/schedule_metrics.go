package metric

import "github.com/allegedlyreliable/sqlstreams/pkg/common/diagnostic"

var MetricOverdueSchedules = diagnostic.NewDiagnosticMetric(
	"SQL0070",
	"sqlstreams.schedule.state.overdue",
	string(MetricKindGauge),
	string(MetricUnitCount("found")),
	"unsuspended schedules due past the overdue threshold",
	diagnostic.MetricScopeSystem,
)

var MetricOldestDueAge = diagnostic.NewDiagnosticMetric(
	"SQL0071",
	"sqlstreams.schedule.state.oldest_due_age",
	string(MetricKindGauge),
	string(MetricUnitMilliseconds),
	"largest time past next scheduled production among unsuspended schedules",
	diagnostic.MetricScopeSystem,
)

var MetricSuspendedSchedules = diagnostic.NewDiagnosticMetric(
	"SQL0072",
	"sqlstreams.schedule.state.suspended",
	string(MetricKindGauge),
	string(MetricUnitCount("found")),
	"schedules excluded from overdue counts because they are suspended",
	diagnostic.MetricScopeSystem,
)
