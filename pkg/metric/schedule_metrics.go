package metric

import "github.com/agentstax/sqlstreams/pkg/common/diagnostic"

var MetricOverdueSchedules = diagnostic.NewDiagnosticMetric(
	"SS0070",
	"sqlstreams.schedule.state.overdue",
	string(MetricKindGauge),
	string(MetricUnitCount("found")),
	"unsuspended schedules due past the overdue threshold",
	diagnostic.MetricScopeSystem,
)

var MetricOldestDueAge = diagnostic.NewDiagnosticMetric(
	"SS0071",
	"sqlstreams.schedule.state.oldest_due_age",
	string(MetricKindGauge),
	string(MetricUnitMilliseconds),
	"largest time past next scheduled production among unsuspended schedules",
	diagnostic.MetricScopeSystem,
)

var MetricSuspendedSchedules = diagnostic.NewDiagnosticMetric(
	"SS0072",
	"sqlstreams.schedule.state.suspended",
	string(MetricKindGauge),
	string(MetricUnitCount("found")),
	"schedules excluded from overdue counts because they are suspended",
	diagnostic.MetricScopeSystem,
)
