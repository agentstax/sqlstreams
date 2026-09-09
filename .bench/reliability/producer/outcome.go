package producer

import (
	"errors"

	"github.com/agentstax/sqlstreams/.bench/reliability/record"
	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/common/diagnostic"
)

// classify sorts a Produce error into the records' outcome. A declared SQL
// error is a rejection the library stands behind, except a lost commit
// confirmation, which is the one declared error that means "unknown".
// Anything else -- a dropped connection, a cancelled ctx at shutdown -- is
// unknown: the row may or may not exist, and the checker finds out.
func classify(err error) (record.ProduceKind, string) {
	if errors.Is(err, common.ErrCommitConfirmationLost) {
		return record.ProduceKindUnknown, ""
	}
	if declared, ok := errors.AsType[*diagnostic.DiagnosticError](err); ok {
		return record.ProduceKindRejected, declared.GetCode()
	}
	return record.ProduceKindUnknown, ""
}
