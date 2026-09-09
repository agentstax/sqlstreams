package metric

import "github.com/agentstax/vulkan/pkg/common/diagnostic"

// These gauges describe an adapter collection, never retained core measurements.
var MetricOTelSourceReadSuccess = diagnostic.NewDiagnosticMetric("VK0102",
	"vulkan.otel.source.read_success", "gauge", "",
	"whether the adapter read retained measurements successfully", diagnostic.MetricScopeExporter)

var MetricOTelMeasurementsRejected = diagnostic.NewDiagnosticMetric("VK0103",
	"vulkan.otel.measurements.rejected", "gauge", "{measurement}",
	"retained measurements omitted from the current export", diagnostic.MetricScopeExporter)
