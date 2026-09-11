package checker

import (
	"testing"

	"github.com/agentstax/sqlstreams/.bench/reliability/scenario"
)

func TestDisabledRecordingDoesNotPassIdentityChecks(t *testing.T) {
	// setup
	checker := &Checker{declared: &scenario.Scenario{DisableMessageRecording: true}}
	expected := scenario.Expectation{Check: scenario.CheckLost, Want: scenario.WantZero}

	// test
	result, err := checker.check(t.Context(), nil, nil, expected)

	// verify
	if err != nil || result.Status != CheckStatusUnavailable || result.Reason == "" {
		t.Errorf("check(lost, recording disabled) = %+v, %v, want unavailable with a reason", result, err)
	}
	if result.ReportColumns() != "UNAVAILABLE\tper-message recording disabled" {
		t.Errorf("ReportColumns(unavailable lost) = %q, want explicit unavailable status", result.ReportColumns())
	}
}
