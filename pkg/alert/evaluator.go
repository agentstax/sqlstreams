package alert

import (
	"context"

	"github.com/agentstax/sqlstreams/pkg/common"
)

// Evaluator validates its owner scope and evaluates retained evidence.
// Threshold 0 uses the alert's live default; read failures return errors.
type Evaluator interface {
	Evaluate(ctx context.Context, owner *common.Owner, policy *JobPayload) (*AlertEvaluationSnapshot, error)
}
