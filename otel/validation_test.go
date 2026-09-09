package otel

import (
	"slices"
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/metric"
)

func TestRejectedFamilies(t *testing.T) {
	tests := []struct {
		name         string
		measurements []metric.Measurement
		want         []string
	}{
		{name: "healthy", measurements: []metric.Measurement{
			{Name: "billing.depth", Kind: metric.MetricKindGauge},
			{Name: "billing.count", Kind: metric.MetricKindCounter},
		}},
		{name: "translated name collision", measurements: []metric.Measurement{
			{Name: "billing.depth", Kind: metric.MetricKindGauge},
			{Name: "billing_depth", Kind: metric.MetricKindGauge},
		}, want: []string{"billing.depth", "billing_depth"}},
		{name: "unit suffix collision", measurements: []metric.Measurement{
			{Name: "billing.duration", Unit: "s", Kind: metric.MetricKindGauge},
			{Name: "billing_duration_seconds", Kind: metric.MetricKindGauge},
		}, want: []string{"billing.duration", "billing_duration_seconds"}},
		{name: "counter suffix collision", measurements: []metric.Measurement{
			{Name: "billing.count", Kind: metric.MetricKindCounter},
			{Name: "billing_count_total", Kind: metric.MetricKindGauge},
		}, want: []string{"billing.count", "billing_count_total"}},
		{name: "kind conflict", measurements: []metric.Measurement{
			{Name: "billing.count", Kind: metric.MetricKindCounter},
			{Name: "billing.count", Kind: metric.MetricKindGauge},
		}, want: []string{"billing.count"}},
		{name: "unit conflict", measurements: []metric.Measurement{
			{Name: "billing.duration", Kind: metric.MetricKindGauge, Unit: "s"},
			{Name: "billing.duration", Kind: metric.MetricKindGauge, Unit: "ms"},
		}, want: []string{"billing.duration"}},
		{name: "invalid family does not reject eligible families", measurements: []metric.Measurement{
			{Name: "billing.count", Kind: metric.MetricKindCounter},
			{Name: "billing.count", Kind: metric.MetricKindGauge},
			{Name: "billing_count_total", Kind: metric.MetricKindGauge},
			{Name: "billing_count", Kind: metric.MetricKindGauge},
			{Name: "healthy_probe", Kind: metric.MetricKindGauge},
		}, want: []string{"billing.count"}},
		{name: "invalid attributes do not reject eligible family", measurements: []metric.Measurement{
			{Name: "billing.depth", Kind: metric.MetricKindGauge, Attributes: map[string]string{"queue.name": "one", "queue_name": "two"}},
			{Name: "billing_depth", Kind: metric.MetricKindGauge},
		}, want: []string{"billing.depth"}},
		{name: "eligible families still collide after invalid family is removed", measurements: []metric.Measurement{
			{Name: "billing.depth", Kind: metric.MetricKindGauge, Attributes: map[string]string{"otel.scope.name": "reserved"}},
			{Name: "billing_depth", Kind: metric.MetricKindGauge},
			{Name: "billing-depth", Kind: metric.MetricKindGauge},
		}, want: []string{"billing.depth", "billing_depth", "billing-depth"}},
		{name: "attribute collision within row", measurements: []metric.Measurement{
			{Name: "billing.depth", Kind: metric.MetricKindGauge, Attributes: map[string]string{"queue.name": "one", "queue_name": "two"}},
		}, want: []string{"billing.depth"}},
		{name: "attribute collision across family", measurements: []metric.Measurement{
			{Name: "billing.depth", Kind: metric.MetricKindGauge, Attributes: map[string]string{"queue.name": "one"}},
			{Name: "billing.depth", Kind: metric.MetricKindGauge, Attributes: map[string]string{"queue_name": "two"}},
		}, want: []string{"billing.depth"}},
		{name: "reserved names", measurements: []metric.Measurement{
			{Name: "sqlstreams_custom", Kind: metric.MetricKindGauge},
			{Name: "target.info", Kind: metric.MetricKindGauge},
			{Name: "otel.scope.info", Kind: metric.MetricKindGauge},
			{Name: metric.MetricOTelSourceReadSuccess.Name, Kind: metric.MetricKindGauge},
			{Name: metric.MetricOTelMeasurementsRejected.Name, Kind: metric.MetricKindGauge, Unit: "{measurement}"},
		}, want: []string{"sqlstreams_custom", "target.info", "otel.scope.info", metric.MetricOTelSourceReadSuccess.Name, metric.MetricOTelMeasurementsRejected.Name}},
		{name: "reserved label", measurements: []metric.Measurement{
			{Name: "billing.depth", Kind: metric.MetricKindGauge, Attributes: map[string]string{"otel.scope.name": "spoof"}},
		}, want: []string{"billing.depth"}},
		{name: "built in remains supported", measurements: []metric.Measurement{
			{Name: metric.MetricCollectorCompletedTimestamp.Name, Kind: metric.MetricKindGauge, Unit: "s"},
		}},
		{name: "untranslatable name", measurements: []metric.Measurement{
			{Name: "...", Kind: metric.MetricKindGauge},
		}, want: []string{"..."}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var rows []*common.StoredMessage[metric.Measurement]
			for _, measurement := range test.measurements {
				measurement.At = time.Now()
				rows = append(rows, &common.StoredMessage[metric.Measurement]{Message: &measurement})
			}
			for range 2 {
				families := make(map[string][]*common.StoredMessage[metric.Measurement])
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
