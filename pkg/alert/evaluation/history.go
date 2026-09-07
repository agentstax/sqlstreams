package evaluation

import (
	"errors"
	"time"

	"github.com/agentstax/vulkan/pkg/alert"
	"github.com/agentstax/vulkan/pkg/common"
)

// EvaluateHistory applies pending to condition results ordered by CreatedAt/id
// descending. The input is read-only; elapsed time alone adds no duration.
func EvaluateHistory(samples []*common.StoredMessage[alert.AlertEvaluationResult], current time.Time, policy *alert.JobPayload) (*alert.AlertEvaluationResult, error) {
	// Validate the resolved policy and evaluation time.
	if policy == nil {
		return nil, errors.New("policy must not be nil")
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if current.IsZero() {
		return nil, errors.New("current must not be zero")
	}

	// Require a fresh observation before evaluating the condition.
	if len(samples) == 0 {
		return alert.NewAlertEvaluationResult(alert.AlertEvaluationStateInsufficientEvidence, nil)
	}
	newest := samples[0]
	if newest.CreatedAt.After(current) {
		return alert.NewAlertEvaluationResult(alert.AlertEvaluationStateInsufficientEvidence, nil)
	}
	if current.Sub(newest.CreatedAt) > policy.MaximumGap {
		return alert.NewAlertEvaluationResult(alert.AlertEvaluationStateInsufficientEvidence, nil)
	}
	if err := newest.Message.Validate(); err != nil {
		return nil, err
	}

	// Only an active condition with pending enabled needs a history scan.
	if newest.Message.State != alert.AlertEvaluationStateActive {
		return newest.Message, nil
	}
	if policy.DisablePending {
		return newest.Message, nil
	}

	// Walk backward through consecutive active observations within the window.
	earliestAt := newest.CreatedAt
	windowStart := current.Add(-policy.Window())
	for _, sample := range samples[1:] {
		// The first row at each timestamp has the highest id.
		if sample.CreatedAt.Equal(earliestAt) {
			continue
		}
		if sample.CreatedAt.Before(windowStart) {
			break
		}
		if earliestAt.Sub(sample.CreatedAt) > policy.MaximumGap {
			break
		}
		if sample.Message.State != alert.AlertEvaluationStateActive {
			break
		}
		earliestAt = sample.CreatedAt
	}

	// Pending duration comes from the observed span, not time spent waiting.
	observedDuration := newest.CreatedAt.Sub(earliestAt)
	if observedDuration < policy.PendingDuration {
		return alert.NewAlertEvaluationResult(alert.AlertEvaluationStatePending, newest.Message.Finding)
	}
	return newest.Message, nil
}
