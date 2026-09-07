package otelvulkan

import (
	"slices"
	"testing"
	"time"

	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/metrics"
)

func TestRejectedFamilies(t *testing.T) {
	tests := []struct {
		name         string
		measurements []metrics.Measurement
		want         []string
	}{
		{name: "healthy", measurements: []metrics.Measurement{
			{Name: "billing.depth", Kind: metrics.MetricKindGauge},
			{Name: "billing.count", Kind: metrics.MetricKindCounter},
		}},
		{name: "translated name collision", measurements: []metrics.Measurement{
			{Name: "billing.depth", Kind: metrics.MetricKindGauge},
			{Name: "billing_depth", Kind: metrics.MetricKindGauge},
		}, want: []string{"billing.depth", "billing_depth"}},
		{name: "unit suffix collision", measurements: []metrics.Measurement{
			{Name: "billing.duration", Unit: "s", Kind: metrics.MetricKindGauge},
			{Name: "billing_duration_seconds", Kind: metrics.MetricKindGauge},
		}, want: []string{"billing.duration", "billing_duration_seconds"}},
		{name: "counter suffix collision", measurements: []metrics.Measurement{
			{Name: "billing.count", Kind: metrics.MetricKindCounter},
			{Name: "billing_count_total", Kind: metrics.MetricKindGauge},
		}, want: []string{"billing.count", "billing_count_total"}},
		{name: "kind conflict", measurements: []metrics.Measurement{
			{Name: "billing.count", Kind: metrics.MetricKindCounter},
			{Name: "billing.count", Kind: metrics.MetricKindGauge},
		}, want: []string{"billing.count"}},
		{name: "unit conflict", measurements: []metrics.Measurement{
			{Name: "billing.duration", Kind: metrics.MetricKindGauge, Unit: "s"},
			{Name: "billing.duration", Kind: metrics.MetricKindGauge, Unit: "ms"},
		}, want: []string{"billing.duration"}},
		{name: "invalid family does not reject eligible families", measurements: []metrics.Measurement{
			{Name: "billing.count", Kind: metrics.MetricKindCounter},
			{Name: "billing.count", Kind: metrics.MetricKindGauge},
			{Name: "billing_count_total", Kind: metrics.MetricKindGauge},
			{Name: "billing_count", Kind: metrics.MetricKindGauge},
			{Name: "healthy_probe", Kind: metrics.MetricKindGauge},
		}, want: []string{"billing.count"}},
		{name: "invalid attributes do not reject eligible family", measurements: []metrics.Measurement{
			{Name: "billing.depth", Kind: metrics.MetricKindGauge, Attributes: map[string]string{"queue.name": "one", "queue_name": "two"}},
			{Name: "billing_depth", Kind: metrics.MetricKindGauge},
		}, want: []string{"billing.depth"}},
		{name: "eligible families still collide after invalid family is removed", measurements: []metrics.Measurement{
			{Name: "billing.depth", Kind: metrics.MetricKindGauge, Attributes: map[string]string{"otel.scope.name": "reserved"}},
			{Name: "billing_depth", Kind: metrics.MetricKindGauge},
			{Name: "billing-depth", Kind: metrics.MetricKindGauge},
		}, want: []string{"billing.depth", "billing_depth", "billing-depth"}},
		{name: "attribute collision within row", measurements: []metrics.Measurement{
			{Name: "billing.depth", Kind: metrics.MetricKindGauge, Attributes: map[string]string{"queue.name": "one", "queue_name": "two"}},
		}, want: []string{"billing.depth"}},
		{name: "attribute collision across family", measurements: []metrics.Measurement{
			{Name: "billing.depth", Kind: metrics.MetricKindGauge, Attributes: map[string]string{"queue.name": "one"}},
			{Name: "billing.depth", Kind: metrics.MetricKindGauge, Attributes: map[string]string{"queue_name": "two"}},
		}, want: []string{"billing.depth"}},
		{name: "reserved names", measurements: []metrics.Measurement{
			{Name: "vulkan_custom", Kind: metrics.MetricKindGauge},
			{Name: "target.info", Kind: metrics.MetricKindGauge},
			{Name: "otel.scope.info", Kind: metrics.MetricKindGauge},
			{Name: metrics.MetricOTelSourceReadSuccess.Name, Kind: metrics.MetricKindGauge},
			{Name: metrics.MetricOTelMeasurementsRejected.Name, Kind: metrics.MetricKindGauge, Unit: "{measurement}"},
		}, want: []string{"vulkan_custom", "target.info", "otel.scope.info", metrics.MetricOTelSourceReadSuccess.Name, metrics.MetricOTelMeasurementsRejected.Name}},
		{name: "reserved label", measurements: []metrics.Measurement{
			{Name: "billing.depth", Kind: metrics.MetricKindGauge, Attributes: map[string]string{"otel.scope.name": "spoof"}},
		}, want: []string{"billing.depth"}},
		{name: "built in remains supported", measurements: []metrics.Measurement{
			{Name: metrics.MetricCollectorCompletedTimestamp.Name, Kind: metrics.MetricKindGauge, Unit: "s"},
		}},
		{name: "untranslatable name", measurements: []metrics.Measurement{
			{Name: "...", Kind: metrics.MetricKindGauge},
		}, want: []string{"..."}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var rows []*common.StoredMessage[metrics.Measurement]
			for _, measurement := range test.measurements {
				measurement.At = time.Now()
				rows = append(rows, &common.StoredMessage[metrics.Measurement]{Message: &measurement})
			}
			for range 2 {
				families := make(map[string][]*common.StoredMessage[metrics.Measurement])
				for _, row := range rows {
					families[row.Message.Name] = append(families[row.Message.Name], row)
				}
				rejected := rejectedFamilies(families)
				if len(rejected) != len(test.want) {
					t.Fatalf("rejected = %v; want %v", rejected, test.want)
				}
				for _, name := range test.want {
					if rejected[name] == "" {
						t.Fatalf("no reason for rejected family %q", name)
					}
				}
				slices.Reverse(rows)
			}
		})
	}
}
