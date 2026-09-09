package evaluation

import (
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/pkg/alert"
	"github.com/agentstax/sqlstreams/pkg/common"
)

// closed set: the pending/active/insufficient outcome for each shape of
// sample history, and the unhealthy span each outcome reports.
func TestEvaluateHistoryStateBySampleShape(t *testing.T) {
	current := time.Date(2026, 9, 7, 10, 2, 0, 0, time.UTC)
	tests := []struct {
		name     string
		states   []alert.AlertEvaluationState
		ages     []time.Duration
		disabled bool
		want     alert.AlertEvaluationState
		wantSpan time.Duration
	}{
		{name: "no samples", want: alert.AlertEvaluationStateInsufficientEvidence},
		{name: "spike", states: []alert.AlertEvaluationState{alert.AlertEvaluationStateActive}, ages: []time.Duration{0}, want: alert.AlertEvaluationStatePending},
		{name: "sustained", states: []alert.AlertEvaluationState{alert.AlertEvaluationStateActive, alert.AlertEvaluationStateActive}, ages: []time.Duration{0, 2 * time.Minute}, want: alert.AlertEvaluationStateActive, wantSpan: 2 * time.Minute},
		{name: "recovery", states: []alert.AlertEvaluationState{alert.AlertEvaluationStateHealthy, alert.AlertEvaluationStateActive}, ages: []time.Duration{0, time.Minute}, want: alert.AlertEvaluationStateHealthy},
		{name: "stale recovery", states: []alert.AlertEvaluationState{alert.AlertEvaluationStateHealthy}, ages: []time.Duration{3 * time.Minute}, want: alert.AlertEvaluationStateInsufficientEvidence},
		{name: "restart gap", states: []alert.AlertEvaluationState{alert.AlertEvaluationStateActive, alert.AlertEvaluationStateActive}, ages: []time.Duration{0, 24 * time.Hour}, want: alert.AlertEvaluationStatePending},
		{name: "gap inside window", states: []alert.AlertEvaluationState{alert.AlertEvaluationStateActive, alert.AlertEvaluationStateActive}, ages: []time.Duration{0, 3 * time.Minute}, want: alert.AlertEvaluationStatePending},
		{name: "insufficient sample breaks span", states: []alert.AlertEvaluationState{alert.AlertEvaluationStateActive, alert.AlertEvaluationStateInsufficientEvidence, alert.AlertEvaluationStateActive}, ages: []time.Duration{0, time.Minute, 2 * time.Minute}, want: alert.AlertEvaluationStatePending},
		{name: "healthy sample breaks span", states: []alert.AlertEvaluationState{alert.AlertEvaluationStateActive, alert.AlertEvaluationStateHealthy, alert.AlertEvaluationStateActive}, ages: []time.Duration{0, time.Minute, 2 * time.Minute}, want: alert.AlertEvaluationStatePending},
		{name: "same instant highest id wins", states: []alert.AlertEvaluationState{alert.AlertEvaluationStateActive, alert.AlertEvaluationStateHealthy, alert.AlertEvaluationStateActive}, ages: []time.Duration{0, 0, 2 * time.Minute}, want: alert.AlertEvaluationStateActive, wantSpan: 2 * time.Minute},
		{name: "waiting adds no duration", states: []alert.AlertEvaluationState{alert.AlertEvaluationStateActive}, ages: []time.Duration{2 * time.Minute}, want: alert.AlertEvaluationStatePending},
		{name: "future storage time", states: []alert.AlertEvaluationState{alert.AlertEvaluationStateActive}, ages: []time.Duration{-time.Minute}, want: alert.AlertEvaluationStateInsufficientEvidence},
		{name: "pending disabled", states: []alert.AlertEvaluationState{alert.AlertEvaluationStateActive}, ages: []time.Duration{0}, disabled: true, want: alert.AlertEvaluationStateActive},
		{name: "pending disabled still needs fresh evidence", states: []alert.AlertEvaluationState{alert.AlertEvaluationStateActive}, ages: []time.Duration{3 * time.Minute}, disabled: true, want: alert.AlertEvaluationStateInsufficientEvidence},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := (&alert.JobPayload{DisablePending: test.disabled}).WithDefaults()
			samples := storedSamples(t, current, test.states, test.ages)

			result, err := EvaluateHistory(samples, current, policy)
			if err != nil {
				t.Fatalf("EvaluateHistory(%s) = %v, want nil", test.name, err)
			}
			if result.State != test.want {
				t.Errorf("EvaluateHistory(%s).State = %s, want %s", test.name, result.State, test.want)
			}
			if result.ObservedDuration != test.wantSpan {
				t.Errorf("EvaluateHistory(%s).ObservedDuration = %v, want %v", test.name, result.ObservedDuration, test.wantSpan)
			}
			if (result.Reason != "") != (result.State == alert.AlertEvaluationStateInsufficientEvidence) {
				t.Errorf("EvaluateHistory(%s).Reason = %q with state %s, want a reason only for insufficient evidence", test.name, result.Reason, result.State)
			}
		})
	}
}

// ***************
// *** HELPERS ***
// ***************

// storedSamples builds one stored evaluation per state, newest first with
// descending ids, each aged back from current.
func storedSamples(t testing.TB, current time.Time, states []alert.AlertEvaluationState, ages []time.Duration) []*common.StoredMessage[alert.AlertEvaluationSnapshot] {
	t.Helper()
	finding := &alert.Alert{Status: alert.AlertStatusActive}
	samples := make([]*common.StoredMessage[alert.AlertEvaluationSnapshot], 0, len(states))
	for i, state := range states {
		var found *alert.Alert
		if state == alert.AlertEvaluationStateActive {
			found = finding
		}
		result, err := alert.NewAlertEvaluationSnapshot(state, found, nil)
		if err != nil {
			t.Fatal(err)
		}
		samples = append(samples, &common.StoredMessage[alert.AlertEvaluationSnapshot]{Id: int64(7106 - i), CreatedAt: current.Add(-ages[i]), Message: result})
	}
	return samples
}
