package evaluation

import (
	"testing"
	"time"

	"github.com/agentstax/vulkan/pkg/alert"
	"github.com/agentstax/vulkan/pkg/common"
)

func TestEvaluateHistory(t *testing.T) {
	current := time.Date(2026, 9, 7, 10, 2, 0, 0, time.UTC)
	finding := &alert.Alert{Status: alert.AlertStatusActive}
	tests := []struct {
		name     string
		states   []alert.AlertEvaluationState
		ages     []time.Duration
		disabled bool
		want     alert.AlertEvaluationState
	}{
		{name: "missing", want: alert.AlertEvaluationStateInsufficientEvidence},
		{name: "spike", states: []alert.AlertEvaluationState{alert.AlertEvaluationStateActive}, ages: []time.Duration{0}, want: alert.AlertEvaluationStatePending},
		{name: "sustained", states: []alert.AlertEvaluationState{alert.AlertEvaluationStateActive, alert.AlertEvaluationStateActive}, ages: []time.Duration{0, 2 * time.Minute}, want: alert.AlertEvaluationStateActive},
		{name: "recovery", states: []alert.AlertEvaluationState{alert.AlertEvaluationStateHealthy, alert.AlertEvaluationStateActive}, ages: []time.Duration{0, time.Minute}, want: alert.AlertEvaluationStateHealthy},
		{name: "stale recovery", states: []alert.AlertEvaluationState{alert.AlertEvaluationStateHealthy}, ages: []time.Duration{3 * time.Minute}, want: alert.AlertEvaluationStateInsufficientEvidence},
		{name: "restart", states: []alert.AlertEvaluationState{alert.AlertEvaluationStateActive, alert.AlertEvaluationStateActive}, ages: []time.Duration{0, 24 * time.Hour}, want: alert.AlertEvaluationStatePending},
		{name: "unusable breaks span", states: []alert.AlertEvaluationState{alert.AlertEvaluationStateActive, alert.AlertEvaluationStateInsufficientEvidence, alert.AlertEvaluationStateActive}, ages: []time.Duration{0, time.Minute, 2 * time.Minute}, want: alert.AlertEvaluationStatePending},
		{name: "tie", states: []alert.AlertEvaluationState{alert.AlertEvaluationStateActive, alert.AlertEvaluationStateHealthy, alert.AlertEvaluationStateActive}, ages: []time.Duration{0, 0, 2 * time.Minute}, want: alert.AlertEvaluationStateActive},
		{name: "no duration by waiting", states: []alert.AlertEvaluationState{alert.AlertEvaluationStateActive}, ages: []time.Duration{2 * time.Minute}, want: alert.AlertEvaluationStatePending},
		{name: "disabled", states: []alert.AlertEvaluationState{alert.AlertEvaluationStateActive}, ages: []time.Duration{0}, disabled: true, want: alert.AlertEvaluationStateActive},
		{name: "future observation", states: []alert.AlertEvaluationState{alert.AlertEvaluationStateActive}, ages: []time.Duration{-time.Minute}, want: alert.AlertEvaluationStateInsufficientEvidence},
		{name: "stale with pending disabled", states: []alert.AlertEvaluationState{alert.AlertEvaluationStateActive}, ages: []time.Duration{3 * time.Minute}, disabled: true, want: alert.AlertEvaluationStateInsufficientEvidence},
		{name: "gap within window", states: []alert.AlertEvaluationState{alert.AlertEvaluationStateActive, alert.AlertEvaluationStateActive}, ages: []time.Duration{0, 3 * time.Minute}, want: alert.AlertEvaluationStatePending},
		{name: "healthy breaks span", states: []alert.AlertEvaluationState{alert.AlertEvaluationStateActive, alert.AlertEvaluationStateHealthy, alert.AlertEvaluationStateActive}, ages: []time.Duration{0, time.Minute, 2 * time.Minute}, want: alert.AlertEvaluationStatePending},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pending := (&alert.JobPayload{DisablePending: test.disabled}).WithDefaults()
			var samples []*common.StoredMessage[alert.AlertEvaluationSnapshot]
			for i, state := range test.states {
				var found *alert.Alert
				if state == alert.AlertEvaluationStateActive {
					found = finding
				}
				result, err := alert.NewAlertEvaluationSnapshot(state, found, nil)
				if err != nil {
					t.Fatal(err)
				}
				samples = append(samples, &common.StoredMessage[alert.AlertEvaluationSnapshot]{Id: int64(7106 - i), CreatedAt: current.Add(-test.ages[i]), Message: result})
			}
			for range 2 {
				result, err := EvaluateHistory(samples, current, pending)
				if err != nil {
					t.Fatal(err)
				}
				if result.State != test.want {
					t.Fatalf("got %s, want %s", result.State, test.want)
				}
				if result.EvaluatedAt != current || result.PendingDuration != pending.PendingDuration || result.MaximumGap != pending.MaximumGap || result.DisablePending != test.disabled || result.MaximumAge != 0 {
					t.Fatalf("snapshot lost evaluation policy: %+v", result)
				}
				if (result.Reason != "") != (result.State == alert.AlertEvaluationStateInsufficientEvidence) {
					t.Fatalf("reason does not match state: %+v", result)
				}
				if len(samples) > 0 && result.ObservedAt != samples[0].CreatedAt {
					t.Fatal("snapshot lost newest observation time")
				}
				if result.State == alert.AlertEvaluationStateHealthy || result.State == alert.AlertEvaluationStateInsufficientEvidence || test.disabled {
					if !result.UnhealthySince.IsZero() || result.ObservedDuration != 0 {
						t.Fatal("snapshot reports an unestablished span")
					}
				} else if result.UnhealthySince.IsZero() || result.ObservedDuration != result.ObservedAt.Sub(result.UnhealthySince) {
					t.Fatal("snapshot span does not match its evidence times")
				}
				if test.name == "sustained" && result.ObservedDuration != 2*time.Minute {
					t.Fatal("snapshot lost the two-minute unhealthy span")
				}
			}
		})
	}
}
