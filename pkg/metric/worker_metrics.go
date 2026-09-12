package metric

import "github.com/allegedlyreliable/sqlstreams/pkg/common/diagnostic"

var MetricUnclaimedWorkers = diagnostic.NewDiagnosticMetric(
	"SQL0067",
	"sqlstreams.worker.state.unclaimed_workers",
	string(MetricKindGauge),
	string(MetricUnitCount("worker")),
	"workers with no live instance and a nonzero target",
	diagnostic.MetricScopeSystem,
)

var MetricOldestUnclaimedAge = diagnostic.NewDiagnosticMetric(
	"SQL0068",
	"sqlstreams.worker.state.oldest_unclaimed_age",
	string(MetricKindGauge),
	string(MetricUnitMilliseconds),
	"largest time since expiry among workers with no live instance and a nonzero target",
	diagnostic.MetricScopeSystem,
)

var MetricFailingWorkers = diagnostic.NewDiagnosticMetric(
	"SQL0069",
	"sqlstreams.worker.state.failing_workers",
	string(MetricKindGauge),
	string(MetricUnitCount("worker")),
	"workers with a live instance on a nonzero consecutive failure streak",
	diagnostic.MetricScopeSystem,
)
