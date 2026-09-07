package evaluation

import (
	"errors"
	"time"

	"github.com/agentstax/vulkan/pkg/alert"
	"github.com/agentstax/vulkan/pkg/common"
)

// EvaluateHistory applies pending to condition results ordered by CreatedAt/id
// descending. The input is read-only; elapsed time alone adds no duration.
func EvaluateHistory(samples []*common.StoredMessage[alert.AlertEvaluationSnapshot], current time.Time, policy *alert.JobPayload) (*alert.AlertEvaluationSnapshot, error) {
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
		return alert.NewAlertEvaluationSnapshot(alert.AlertEvaluationStateInsufficientEvidence, nil, &alert.AlertEvaluationSnapshotConfig{
			EvaluatedAt:     current,
			PendingDuration: policy.PendingDuration,
			MaximumGap:      policy.MaximumGap,
			DisablePending:  policy.DisablePending,
			Reason:          "no retained measurement",
		})
	}
	newest := samples[0]
	var reason string
	switch {
	case newest.CreatedAt.After(current):
		reason = "measurement storage time is after evaluation time"
	case current.Sub(newest.CreatedAt) > policy.MaximumGap:
		reason = "newest measurement exceeds MaximumGap"
	default:
		if err := newest.Message.Validate(); err != nil {
			return nil, err
		}
		if newest.Message.State == alert.AlertEvaluationStateInsufficientEvidence {
			reason = "newest measurement cannot establish the alert condition"
		}
	}
	if reason != "" {
		return alert.NewAlertEvaluationSnapshot(alert.AlertEvaluationStateInsufficientEvidence, nil, &alert.AlertEvaluationSnapshotConfig{
			EvaluatedAt:     current,
			ObservedAt:      newest.CreatedAt,
			PendingDuration: policy.PendingDuration,
			MaximumGap:      policy.MaximumGap,
			DisablePending:  policy.DisablePending,
			Reason:          reason,
		})
	}

	// Only an active condition with pending enabled needs a history scan.
	if newest.Message.State != alert.AlertEvaluationStateActive || policy.DisablePending {
		return alert.NewAlertEvaluationSnapshot(newest.Message.State, newest.Message.Finding, &alert.AlertEvaluationSnapshotConfig{
			EvaluatedAt:     current,
			ObservedAt:      newest.CreatedAt,
			PendingDuration: policy.PendingDuration,
			MaximumGap:      policy.MaximumGap,
			DisablePending:  policy.DisablePending,
		})
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
	state := alert.AlertEvaluationStateActive
	if observedDuration < policy.PendingDuration {
		state = alert.AlertEvaluationStatePending
	}
	return alert.NewAlertEvaluationSnapshot(state, newest.Message.Finding, &alert.AlertEvaluationSnapshotConfig{
		EvaluatedAt:      current,
		ObservedAt:       newest.CreatedAt,
		UnhealthySince:   earliestAt,
		ObservedDuration: observedDuration,
		PendingDuration:  policy.PendingDuration,
		MaximumGap:       policy.MaximumGap,
		DisablePending:   policy.DisablePending,
	})
}
