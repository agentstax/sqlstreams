package otel

import (
	"time"

	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/common/diagnostic"
	"github.com/agentstax/vulkan/pkg/metric"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func toCollection(accepted []*common.StoredMessage[metric.Measurement], measurementCount int, readSucceeded bool) []metricdata.ScopeMetrics {
	current := time.Now()
	var collected []metricdata.Metrics
	if readSucceeded {
		collected = toMetrics(accepted, current)
		collected = append(collected,
			toCollectionGauge(metric.MetricOTelSourceReadSuccess, 1, current),
			toCollectionGauge(metric.MetricOTelMeasurementsRejected, float64(measurementCount-len(accepted)), current),
		)
	} else {
		collected = []metricdata.Metrics{toCollectionGauge(metric.MetricOTelSourceReadSuccess, 0, current)}
	}
	return []metricdata.ScopeMetrics{{
		Scope:   instrumentation.Scope{Name: meterScopeName},
		Metrics: collected,
	}}
}

func toCollectionGauge(declared *diagnostic.DiagnosticMetric, value float64, current time.Time) metricdata.Metrics {
	return metricdata.Metrics{
		Name:        declared.Name,
		Unit:        declared.Unit,
		Description: declared.Description,
		Data:        metricdata.Gauge[float64]{DataPoints: []metricdata.DataPoint[float64]{{Time: current, Value: value}}},
	}
}

func toMetrics(rows []*common.StoredMessage[metric.Measurement], current time.Time) []metricdata.Metrics {
	positions := make(map[[3]string]int)
	collected := make([]metricdata.Metrics, 0)
	for _, row := range rows {
		measurement := row.Message
		identity := [3]string{measurement.Name, string(measurement.Kind), string(measurement.Unit)}
		position, found := positions[identity]
		if !found {
			position = len(collected)
			positions[identity] = position
			metric := metricdata.Metrics{Name: measurement.Name, Unit: string(measurement.Unit)}
			if declared, found := diagnostic.GetMetric(measurement.Name); found {
				metric.Description = declared.Description
			}
			collected = append(collected, metric)
		}
		point := metricdata.DataPoint[float64]{
			Attributes: attribute.NewSet(toAttributes(measurement.Attributes)...),
			Time:       current,
			Value:      measurement.Value,
		}
		switch measurement.Kind {
		case metric.MetricKindCounter:
			// Retained totals have no established start time; exporter startup is not a reset.
			sum, _ := collected[position].Data.(metricdata.Sum[float64])
			sum.Temporality = metricdata.CumulativeTemporality
			sum.IsMonotonic = true
			sum.DataPoints = append(sum.DataPoints, point)
			collected[position].Data = sum
		default:
			gauge, _ := collected[position].Data.(metricdata.Gauge[float64])
			gauge.DataPoints = append(gauge.DataPoints, point)
			collected[position].Data = gauge
		}
	}
	return collected
}

func toAttributes(attributes map[string]string) []attribute.KeyValue {
	pairs := make([]attribute.KeyValue, 0, len(attributes))
	for key, value := range attributes {
		pairs = append(pairs, attribute.String(key, value))
	}
	return pairs
}
