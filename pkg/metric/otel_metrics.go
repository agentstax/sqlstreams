package metric

import "github.com/agentstax/sqlstreams/pkg/common/diagnostic"

// These gauges describe an adapter collection, never retained core measurements.
var MetricOTelSourceReadSuccess = diagnostic.NewDiagnosticMetric("SS0102",
	"sqlstreams.otel.source.read_success", "gauge", "",
	"whether the adapter read retained measurements successfully", diagnostic.MetricScopeExporter)

var MetricOTelMeasurementsRejected = diagnostic.NewDiagnosticMetric("SS0103",
	"sqlstreams.otel.measurements.rejected", "gauge", "{measurement}",
	"retained measurements omitted from the current export", diagnostic.MetricScopeExporter)
