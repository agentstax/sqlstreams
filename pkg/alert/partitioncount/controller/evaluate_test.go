package controller

import (
	"math"
	"testing"
	"time"

	"github.com/agentstax/vulkan/pkg/alert"
	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/metrics"
)

func TestEvaluateValidatesBeforeReading(t *testing.T) {
	owner, err := common.NewTopicOwner(1, 41, "orders")
	if err != nil {
		t.Fatal(err)
	}
	controller := &PartitionCountController{}
	if _, err := controller.Evaluate(t.Context(), nil, &alert.JobPayload{}); err == nil {
		t.Fatal("nil owner must fail before database access")
	}
	if _, err := controller.Evaluate(t.Context(), owner, &alert.JobPayload{Threshold: -1}); err == nil {
		t.Fatal("negative threshold must fail before database access")
	}
	if _, err := controller.Evaluate(t.Context(), owner, nil); err == nil {
		t.Fatal("nil policy must fail before database access")
	}
}

func TestEvaluateHistory(t *testing.T) {
	current := time.Date(2026, time.September, 7, 10, 2, 0, 0, time.UTC)
	owner, err := common.NewTopicOwner(1, 41, "orders")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		ages      []time.Duration
		values    []float64
		disabled  bool
		threshold int64
		state     alert.AlertEvaluationState
	}{
		{name: "missing", state: alert.AlertEvaluationStateInsufficientEvidence},
		{name: "healthy", ages: []time.Duration{0}, values: []float64{95}, state: alert.AlertEvaluationStateHealthy},
		{name: "first crossing", ages: []time.Duration{0}, values: []float64{100}, state: alert.AlertEvaluationStatePending},
		{name: "one minute", ages: []time.Duration{0, time.Minute}, values: []float64{110, 105}, state: alert.AlertEvaluationStatePending},
		{name: "two minutes", ages: []time.Duration{0, time.Minute, 2 * time.Minute}, values: []float64{110, 108, 105}, state: alert.AlertEvaluationStateActive},
		{name: "healthy interrupts", ages: []time.Duration{0, time.Minute, 2 * time.Minute}, values: []float64{110, 95, 105}, state: alert.AlertEvaluationStatePending},
		{name: "gap interrupts", ages: []time.Duration{0, 3 * time.Minute}, values: []float64{110, 105}, state: alert.AlertEvaluationStatePending},
		{name: "exact gap allowed", ages: []time.Duration{0, 2 * time.Minute}, values: []float64{110, 105}, state: alert.AlertEvaluationStateActive},
		{name: "exact freshness allowed", ages: []time.Duration{2 * time.Minute, 4 * time.Minute}, values: []float64{110, 105}, state: alert.AlertEvaluationStateActive},
		{name: "stale unhealthy", ages: []time.Duration{2*time.Minute + time.Microsecond}, values: []float64{110}, state: alert.AlertEvaluationStateInsufficientEvidence},
		{name: "stale healthy", ages: []time.Duration{3 * time.Minute}, values: []float64{95}, state: alert.AlertEvaluationStateInsufficientEvidence},
		{name: "one sample cannot age into active", ages: []time.Duration{2 * time.Minute}, values: []float64{110}, state: alert.AlertEvaluationStatePending},
		{name: "overnight restart", ages: []time.Duration{0, 24 * time.Hour}, values: []float64{110, 105}, state: alert.AlertEvaluationStatePending},
		{name: "unusable newest", ages: []time.Duration{0, time.Minute, 2 * time.Minute}, values: []float64{math.NaN(), 110, 105}, state: alert.AlertEvaluationStateInsufficientEvidence},
		{name: "unusable interrupts", ages: []time.Duration{0, time.Minute, 2 * time.Minute}, values: []float64{110, -1, 105}, state: alert.AlertEvaluationStatePending},
		{name: "fractional count", ages: []time.Duration{0}, values: []float64{100.5}, state: alert.AlertEvaluationStateInsufficientEvidence},
		{name: "infinite count", ages: []time.Duration{0}, values: []float64{math.Inf(1)}, state: alert.AlertEvaluationStateInsufficientEvidence},
		{name: "out of range count", ages: []time.Duration{0}, values: []float64{float64(math.MaxInt64)}, state: alert.AlertEvaluationStateInsufficientEvidence},
		{name: "future storage time", ages: []time.Duration{-time.Minute}, values: []float64{110}, state: alert.AlertEvaluationStateInsufficientEvidence},
		{name: "disabled", ages: []time.Duration{0}, values: []float64{110}, disabled: true, state: alert.AlertEvaluationStateActive},
		{name: "disabled still needs fresh evidence", ages: []time.Duration{3 * time.Minute}, values: []float64{110}, disabled: true, state: alert.AlertEvaluationStateInsufficientEvidence},
		{name: "tie highest id wins", ages: []time.Duration{0, time.Minute, time.Minute, 2 * time.Minute}, values: []float64{110, 108, 95, 105}, state: alert.AlertEvaluationStateActive},
		{name: "tie highest id healthy breaks", ages: []time.Duration{0, time.Minute, time.Minute, 2 * time.Minute}, values: []float64{110, 95, 108, 105}, state: alert.AlertEvaluationStatePending},
		{name: "same time adds no duration", ages: []time.Duration{0, 0, 0}, values: []float64{110, 108, 105}, state: alert.AlertEvaluationStatePending},
		{name: "changed threshold reuses evidence", ages: []time.Duration{0, time.Minute, 2 * time.Minute}, values: []float64{110, 108, 105}, threshold: 120, state: alert.AlertEvaluationStateHealthy},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy, err := alert.NewJobPayload(test.threshold, 0, 0, test.disabled)
			if err != nil {
				t.Fatal(err)
			}
			history := &metrics.MeasurementHistory{EvaluatedAt: current}
			for i, age := range test.ages {
				history.Messages = append(history.Messages, &common.StoredMessage[metrics.Measurement]{
					Id: int64(7104 - i), CreatedAt: current.Add(-age),
					Message: &metrics.Measurement{Name: metrics.MetricTopicPartitions.Name, Kind: metrics.MetricKindGauge, Unit: metrics.MetricUnit(metrics.MetricTopicPartitions.Unit), Value: test.values[i], At: current.Add(-24 * time.Hour)},
				})
			}
			controller := &PartitionCountController{}
			for range 2 {
				result, err := controller.evaluateHistory(owner, policy, 200, history)
				if err != nil {
					t.Fatal(err)
				}
				if result.State != test.state {
					t.Fatalf("state = %q, want %q", result.State, test.state)
				}
				if result.Finding != nil && (!result.Finding.At.Equal(history.Messages[0].CreatedAt) || result.Finding.Owner != owner) {
					t.Fatalf("finding must describe newest stored observation: %+v", result.Finding)
				}
			}
		})
	}
}

func TestEmptyMeasurementIsNotHealthy(t *testing.T) {
	owner, err := common.NewTopicOwner(1, 41, "orders")
	if err != nil {
		t.Fatal(err)
	}
	controller := &PartitionCountController{}
	result, err := controller.evaluateMeasurement(owner, 100, 200, &metrics.Measurement{}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if result.State != alert.AlertEvaluationStateInsufficientEvidence {
		t.Fatal("a decoded null or empty payload must not establish health")
	}
}
