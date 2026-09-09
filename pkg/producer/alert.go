package producer

import (
	"context"
	"errors"

	"github.com/agentstax/sqlstreams/pkg/alert"
	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/common/logging"
	"github.com/agentstax/sqlstreams/pkg/stream"
)

// logAlerts measures the stream against the same conditions the system's
// alert jobs evaluate on a schedule, and logs any that hold. Log-only:
//   - a register path never writes alerts
//   - a failed measure never fails Register
func (p *Producer) logAlerts(ctx context.Context, current *stream.Stream, logger logging.Logger, evaluators []alert.Evaluator) {
	owner, err := common.NewStreamOwner(current.SystemId, current.Id, current.Name)
	if err != nil {
		logger.WarnContext(ctx, "could not run register-time alert pass", "stream", current.Name, "error", err)
		return
	}

	for _, evaluator := range evaluators {
		policy, err := alert.NewJobPayload(0, 0, 0, true)
		if err != nil {
			logger.WarnContext(ctx, "could not run register-time alert pass", "stream", current.Name, "error", err)
			continue
		}
		result, err := evaluator.Evaluate(ctx, owner, policy)
		if err != nil {
			logger.WarnContext(ctx, "could not run register-time alert pass", "stream", current.Name, "error", err)
			continue
		}
		if result.State == alert.AlertEvaluationStateInsufficientEvidence {
			logger.WarnContext(ctx, "could not run register-time alert pass", "stream", current.Name, "error", errors.New("alert evidence is insufficient"))
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
