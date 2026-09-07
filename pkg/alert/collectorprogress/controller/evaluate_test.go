package controller

import (
	"testing"
	"time"

	"github.com/agentstax/vulkan/pkg/alert"
	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/metrics"
	"github.com/agentstax/vulkan/pkg/worker"
)

func TestCollectorProgressHistory(t *testing.T) {
	current := time.Date(2026, 9, 7, 10, 5, 0, 0, time.UTC)
	owner, err := common.NewSystemOwner(1)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name          string
		completionAge time.Duration
		missing       bool
		leases        [][2]time.Duration
		disabled      bool
		want          alert.AlertEvaluationState
	}{
		{name: "recent completion", completionAge: time.Minute, want: alert.AlertEvaluationStateHealthy},
		{name: "overdue no coverage", completionAge: 5 * time.Minute, want: alert.AlertEvaluationStateInsufficientEvidence},
		{name: "expired manager", completionAge: 5 * time.Minute, leases: [][2]time.Duration{{-5 * time.Minute, 0}}, want: alert.AlertEvaluationStateInsufficientEvidence},
		{name: "first overdue", completionAge: 2 * time.Minute, leases: [][2]time.Duration{{-5 * time.Minute, time.Second}}, want: alert.AlertEvaluationStatePending},
		{name: "sustained", completionAge: 4 * time.Minute, leases: [][2]time.Duration{{-5 * time.Minute, time.Second}}, want: alert.AlertEvaluationStateActive},
		{name: "recent manager limits span", completionAge: 24 * time.Hour, leases: [][2]time.Duration{{-time.Minute, time.Second}}, want: alert.AlertEvaluationStatePending},
		{name: "overlap preserves pending", completionAge: 5 * time.Minute, leases: [][2]time.Duration{{-time.Minute, time.Second}, {-5 * time.Minute, -30 * time.Second}}, want: alert.AlertEvaluationStateActive},
		{name: "touching preserves pending", completionAge: 5 * time.Minute, leases: [][2]time.Duration{{-time.Minute, time.Second}, {-5 * time.Minute, -time.Minute}}, want: alert.AlertEvaluationStateActive},
		{name: "gap breaks pending", completionAge: 5 * time.Minute, leases: [][2]time.Duration{{-time.Minute, time.Second}, {-5 * time.Minute, -2 * time.Minute}}, want: alert.AlertEvaluationStatePending},
		{name: "overnight restart", completionAge: 25 * time.Hour, leases: [][2]time.Duration{{-time.Minute, time.Second}, {-26 * time.Hour, -24 * time.Hour}}, want: alert.AlertEvaluationStatePending},
		{name: "older instance bridges shorter one", completionAge: 5 * time.Minute, leases: [][2]time.Duration{{-time.Minute, time.Second}, {-2 * time.Minute, -90 * time.Second}, {-5 * time.Minute, -30 * time.Second}}, want: alert.AlertEvaluationStateActive},
		{name: "no completion with coverage", missing: true, leases: [][2]time.Duration{{-2 * time.Minute, time.Second}}, want: alert.AlertEvaluationStateActive},
		{name: "no completion new manager", missing: true, leases: [][2]time.Duration{{-time.Minute, time.Second}}, want: alert.AlertEvaluationStatePending},
		{name: "no completion no history", missing: true, want: alert.AlertEvaluationStateInsufficientEvidence},
		{name: "disabled pending", completionAge: 5 * time.Minute, leases: [][2]time.Duration{{-time.Second, time.Second}}, disabled: true, want: alert.AlertEvaluationStateActive},
		{name: "disabled still requires coverage", completionAge: 5 * time.Minute, disabled: true, want: alert.AlertEvaluationStateInsufficientEvidence},
		{name: "future completion", completionAge: -time.Minute, leases: [][2]time.Duration{{-5 * time.Minute, time.Second}}, want: alert.AlertEvaluationStateInsufficientEvidence},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := (&alert.JobPayload{DisablePending: test.disabled}).WithDefaults()
			var completion *common.StoredMessage[metrics.Measurement]
			if !test.missing {
				at := current.Add(-test.completionAge)
				measurement, err := metrics.NewBuiltInMeasurement(metrics.MetricCollectorCompletedTimestamp, float64(at.Unix()), nil, at)
				if err != nil {
					t.Fatal(err)
				}
				completion = &common.StoredMessage[metrics.Measurement]{Id: 7101, CreatedAt: at, Message: measurement}
			}
			history := &worker.WorkerInstanceHistory{EvaluatedAt: current}
			for _, lease := range test.leases {
				history.Instances = append(history.Instances, worker.WorkerInstanceSnapshot{CreatedAt: current.Add(lease[0]), ExpiresAt: current.Add(lease[1])})
			}
			controller := &CollectorProgressController{}
			for range 2 {
				result, err := controller.evaluateHistory(owner, completion, history, 2*time.Minute, policy)
				if err != nil {
					t.Fatal(err)
				}
				if result.State != test.want {
					t.Fatalf("state = %s, want %s", result.State, test.want)
				}
				if result.EvaluatedAt != current || result.PendingDuration != policy.PendingDuration || result.MaximumAge != 2*time.Minute || result.MaximumGap != 0 || result.DisablePending != test.disabled {
					t.Fatalf("snapshot lost evaluation policy: %+v", result)
				}
				if (result.Reason != "") != (result.State == alert.AlertEvaluationStateInsufficientEvidence) {
					t.Fatalf("reason does not match state: %+v", result)
				}
				if test.missing && !result.ObservedAt.IsZero() {
					t.Fatal("missing completion has an observation time")
				}
				if result.State == alert.AlertEvaluationStateHealthy || result.State == alert.AlertEvaluationStateInsufficientEvidence {
					if !result.UnhealthySince.IsZero() || result.ObservedDuration != 0 {
						t.Fatal("snapshot reports an unestablished span")
					}
				} else if result.UnhealthySince.IsZero() || result.ObservedDuration != current.Sub(result.UnhealthySince) {
					t.Fatal("snapshot span does not match its evaluation times")
				}
				if test.name == "recent manager limits span" && result.ObservedDuration != time.Minute {
					t.Fatal("snapshot counted time before manager coverage")
				}
				if result.Finding != nil && (result.Finding.Owner != owner || result.Finding.At != current) {
					t.Fatal("finding lost system owner or evaluation time")
				}
			}
		})
	}
}
