package alert

import "testing"

func TestAlertEvaluationSnapshot(t *testing.T) {
	tests := []struct {
		name      string
		state     AlertEvaluationState
		finding   *Alert
		wantError bool
	}{
		{name: "healthy", state: AlertEvaluationStateHealthy},
		{name: "insufficient", state: AlertEvaluationStateInsufficientEvidence},
		{name: "pending", state: AlertEvaluationStatePending, finding: &Alert{Status: AlertStatusActive}},
		{name: "active", state: AlertEvaluationStateActive, finding: &Alert{Status: AlertStatusActive}},
		{name: "empty state", wantError: true},
		{name: "unrecognized state", state: "other", wantError: true},
		{name: "healthy finding", state: AlertEvaluationStateHealthy, finding: &Alert{Status: AlertStatusActive}, wantError: true},
		{name: "insufficient finding", state: AlertEvaluationStateInsufficientEvidence, finding: &Alert{Status: AlertStatusActive}, wantError: true},
		{name: "pending without finding", state: AlertEvaluationStatePending, wantError: true},
		{name: "active without finding", state: AlertEvaluationStateActive, wantError: true},
		{name: "resolved finding", state: AlertEvaluationStateActive, finding: &Alert{Status: AlertStatusResolved}, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := NewAlertEvaluationSnapshot(test.state, test.finding, nil)
			if test.wantError {
				if err == nil || result != nil {
					t.Fatalf("got %v, %v; want nil result and error", result, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.State != test.state || result.Finding != test.finding {
				t.Fatalf("result = %+v", result)
			}
		})
	}
}
