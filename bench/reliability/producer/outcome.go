package producer

import (
	"errors"

	"github.com/agentstax/vulkan/bench/reliability/ledger"
	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/common/diagnostic"
)

// classify sorts a Produce error into the ledger's outcome. A declared VK
// error is a rejection the library stands behind, except a lost commit
// confirmation, which is the one declared error that means "unknown".
// Anything else -- a dropped connection, a cancelled ctx at shutdown -- is
// unknown: the row may or may not exist, and the checker finds out.
func classify(err error) (ledger.ProduceKind, string) {
	if errors.Is(err, common.ErrCommitConfirmationLost) {
		return ledger.ProduceUnknown, ""
	}
	if declared, ok := errors.AsType[*diagnostic.DiagnosticError](err); ok {
		return ledger.ProduceRejected, declared.GetCode()
	}
	return ledger.ProduceUnknown, ""
}
