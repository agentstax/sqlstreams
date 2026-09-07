package alert

import (
	"context"

	"github.com/agentstax/vulkan/pkg/common"
)

// Evaluator returns an explicit condition result per owner topic.
// Threshold 0 uses the alert's live default; read failures return errors.
type Evaluator interface {
	Evaluate(ctx context.Context, owner *common.Owner, policy *JobPayload) (*AlertEvaluationResult, error)
}
