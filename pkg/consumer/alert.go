package consumer

import (
	"context"
	"errors"

	"github.com/agentstax/vulkan/pkg/alert"
	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/common/logging"
	"github.com/agentstax/vulkan/pkg/topic"
)

// logAlerts measures the topic against the same conditions the system's
// alert jobs evaluate on a schedule, and logs any that hold. Log-only:
//   - a register path never writes alerts
//   - a failed measure never fails Register
func (c *Consumer) logAlerts(ctx context.Context, current *topic.Topic, logger logging.Logger, evaluators []alert.Evaluator) {
	owner, err := common.NewTopicOwner(current.SystemId, current.Id, current.Name)
	if err != nil {
		logger.WarnContext(ctx, "could not run register-time alert pass", "topic", current.Name, "error", err)
		return
	}

	for _, evaluator := range evaluators {
		policy, err := alert.NewJobPayload(0, 0, 0, true)
		if err != nil {
			logger.WarnContext(ctx, "could not run register-time alert pass", "topic", current.Name, "error", err)
			continue
		}
		result, err := evaluator.Evaluate(ctx, owner, policy)
		if err != nil {
			logger.WarnContext(ctx, "could not run register-time alert pass", "topic", current.Name, "error", err)
			continue
		}
		if result.State == alert.AlertEvaluationStateInsufficientEvidence {
			logger.WarnContext(ctx, "could not run register-time alert pass", "topic", current.Name, "error", errors.New("alert evidence is insufficient"))
			continue
		}
		if result.State != alert.AlertEvaluationStateActive && result.State != alert.AlertEvaluationStatePending {
			continue
		}
		found := result.Finding
		logger.WarnContext(ctx, alert.EventAlertConditionHolds.Message(),
			"code", alert.EventAlertConditionHolds.GetCode(),
			"alert", found.Name, "alert_message", found.Message,
			"detail", found.Detail, "hint", found.Hint,
			"owner", found.Owner.Name, "severity", found.Severity)
	}
}
