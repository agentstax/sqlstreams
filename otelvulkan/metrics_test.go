package otelvulkan

import (
	"testing"
	"time"

	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/metrics"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestRetainedMetricConversion(t *testing.T) {
	current := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	var rows []*common.StoredMessage[metrics.Measurement]
	for _, test := range []struct {
		name   string
		kind   metrics.MetricKind
		unit   metrics.MetricUnit
		value  float64
		worker string
	}{
		{"queue_depth", metrics.MetricKindGauge, "{message}", 7, "first"},
		{"queue_depth", metrics.MetricKindGauge, "{message}", 9, "second"},
		{"handled", metrics.MetricKindCounter, "{message}", 120, "first"},
	} {
		measurement, err := metrics.NewMeasurement(test.name, test.kind, test.value, test.unit, map[string]string{"worker": test.worker}, current.Add(-time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		measurement.Metadata = []byte(`{"private":"not exported"}`)
		rows = append(rows, &common.StoredMessage[metrics.Measurement]{Message: measurement})
	}
	for range 2 {
		collected := toMetrics(rows, current)
		if len(collected) != 2 {
			t.Fatalf("metric families = %d", len(collected))
		}
		gauge := collected[0].Data.(metricdata.Gauge[float64])
		if len(gauge.DataPoints) != 2 || gauge.DataPoints[0].Value != 7 || gauge.DataPoints[1].Value != 9 || gauge.DataPoints[0].Attributes.Len() != 1 {
			t.Fatalf("gauge = %+v", gauge)
		}
		counter := collected[1].Data.(metricdata.Sum[float64])
		if !counter.IsMonotonic || counter.Temporality != metricdata.CumulativeTemporality || counter.DataPoints[0].Value != 120 {
			t.Fatalf("counter = %+v", counter)
		}
		if !counter.DataPoints[0].StartTime.IsZero() || counter.DataPoints[0].Time != current {
			t.Fatal("counter invented a reset or lost collection time")
		}
	}
	if len(toMetrics(nil, current)) != 0 {
		t.Fatal("empty source invented a series")
	}
}
