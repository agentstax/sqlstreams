package alert

import (
	"errors"
	"fmt"
)

// AlertEvaluationState describes evidence, not a recorded alert's lifecycle.
type AlertEvaluationState string

const (
	AlertEvaluationStateHealthy              AlertEvaluationState = "healthy"
	AlertEvaluationStatePending              AlertEvaluationState = "pending"
	AlertEvaluationStateActive               AlertEvaluationState = "active"
	AlertEvaluationStateInsufficientEvidence AlertEvaluationState = "insufficient_evidence"
)

// AlertEvaluationResult distinguishes recovery from evidence that cannot
// activate or resolve an alert. Finding is present only for pending and active.
type AlertEvaluationResult struct {
	State   AlertEvaluationState `json:"state"`
	Finding *Alert               `json:"finding"`
}

func NewAlertEvaluationResult(state AlertEvaluationState, finding *Alert) (*AlertEvaluationResult, error) {
	result := &AlertEvaluationResult{State: state, Finding: finding}
	if err := result.Validate(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *AlertEvaluationResult) Validate() error {
	switch r.State {
	case AlertEvaluationStateHealthy, AlertEvaluationStateInsufficientEvidence:
		if r.Finding != nil {
			return fmt.Errorf("Finding must be nil for state %q", r.State)
		}
	case AlertEvaluationStatePending, AlertEvaluationStateActive:
		if r.Finding == nil {
			return errors.New("Finding must not be nil")
		}
		if r.Finding.Status != AlertStatusActive {
			return fmt.Errorf("Finding.Status must be %q, got %q", AlertStatusActive, r.Finding.Status)
		}
	default:
		return fmt.Errorf("unrecognized alert evaluation state: %q", r.State)
	}
	return nil
}
