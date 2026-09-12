package controller

import (
	"math"
	"testing"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/alert"
	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/metric"
)

// closed set: how one retained partition-count measurement reads against the
// threshold -- a value that is not a whole non-negative count, or not the
// partitions gauge at all, is insufficient evidence rather than healthy.
func TestEvaluateMeasurementStateByValue(t *testing.T) {
	owner, err := common.NewStreamOwner(1, 41, "orders")
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 7, 10, 2, 0, 0, time.UTC)
	tests := []struct {
		name        string
		measurement *metric.Measurement
		want        alert.AlertEvaluationState
	}{
		{name: "below threshold", measurement: partitionsGauge(95), want: alert.AlertEvaluationStateHealthy},
		{name: "at threshold", measurement: partitionsGauge(100), want: alert.AlertEvaluationStateActive},
		{name: "above threshold", measurement: partitionsGauge(110), want: alert.AlertEvaluationStateActive},
		{name: "not a number", measurement: partitionsGauge(math.NaN()), want: alert.AlertEvaluationStateInsufficientEvidence},
		{name: "negative", measurement: partitionsGauge(-1), want: alert.AlertEvaluationStateInsufficientEvidence},
		{name: "fractional", measurement: partitionsGauge(100.5), want: alert.AlertEvaluationStateInsufficientEvidence},
		{name: "infinite", measurement: partitionsGauge(math.Inf(1)), want: alert.AlertEvaluationStateInsufficientEvidence},
		{name: "beyond int64", measurement: partitionsGauge(float64(math.MaxInt64)), want: alert.AlertEvaluationStateInsufficientEvidence},
		{name: "empty measurement", measurement: &metric.Measurement{}, want: alert.AlertEvaluationStateInsufficientEvidence},
		{name: "another metric", measurement: &metric.Measurement{Name: metric.MetricStreamCompacted.Name, Kind: metric.MetricKindGauge, Value: 110}, want: alert.AlertEvaluationStateInsufficientEvidence},
	}
	controller := &PartitionCountController{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := controller.evaluateMeasurement(owner, 100, 200, test.measurement, at)
			if err != nil {
				t.Fatalf("evaluateMeasurement(%s) = %v, want nil", test.name, err)
			}
			if result.State != test.want {
				t.Fatalf("evaluateMeasurement(%s).State = %s, want %s", test.name, result.State, test.want)
			}
		})
	}
}

// behavior: a zero threshold reads as half the live lock ceiling, so the
// alert activates with headroom before Destroy starts failing.
func TestZeroThresholdReadsAsHalfTheLockCeiling(t *testing.T) {
	owner, err := common.NewStreamOwner(1, 41, "orders")
	if err != nil {
		t.Fatal(err)
	}
	policy, err := alert.NewJobPayload(0, 0, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 7, 10, 2, 0, 0, time.UTC)
	tests := []struct {
		name  string
		count float64
		want  alert.AlertEvaluationState
	}{
		{name: "below half", count: 99, want: alert.AlertEvaluationStateHealthy},
		{name: "at half", count: 100, want: alert.AlertEvaluationStateActive},
	}
	controller := &PartitionCountController{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			history := &metric.MeasurementHistory{EvaluatedAt: at, Messages: []*common.StoredMessage[metric.Measurement]{
				{Id: 7104, CreatedAt: at, Message: partitionsGauge(test.count)},
			}}

			result, err := controller.evaluateHistory(owner, policy, 200, history)
			if err != nil {
				t.Fatalf("evaluateHistory(%s) = %v, want nil", test.name, err)
			}
			if result.State != test.want {
				t.Fatalf("evaluateHistory(%s).State = %s, want %s", test.name, result.State, test.want)
			}
		})
	}
}

// ***************
// *** HELPERS ***
// ***************

func partitionsGauge(value float64) *metric.Measurement {
	return &metric.Measurement{
		Name:  metric.MetricStreamPartitions.Name,
		Kind:  metric.MetricKindGauge,
		Unit:  metric.MetricUnit(metric.MetricStreamPartitions.Unit),
		Value: value,
	}
}
