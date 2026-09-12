package worker

import (
	"github.com/allegedlyreliable/sqlstreams/pkg/common/diagnostic"
)

// ErrInstanceLost means the instance row expired or was removed mid-work:
// stop -- a replacement may already be running.
var ErrInstanceLost = diagnostic.NewDiagnosticError("SQL0012", diagnostic.RecoveryPermanent,
	"worker instance row expired or was removed",
	"stop the work; a replacement may already be running")

// ErrWorkerDeclarationInterrupted means the worker row was deleted between the
// declaration's insert attempt and its update; an unchanged retry re-creates
// the row, so DatastoreRetry heals the race.
var ErrWorkerDeclarationInterrupted = diagnostic.NewDiagnosticError("SQL0024", diagnostic.RecoveryTransient,
	"could not finish the worker declaration",
	"rerun the declaration if the worker should still exist")

// ErrWorkerNotFound means the requested worker has not been declared.
var ErrWorkerNotFound = diagnostic.NewDiagnosticError("SQL0106", diagnostic.RecoveryPermanent,
	"worker not found", "register the owning resource before operating its worker")

// ErrWorkerSuspended stops a running execution after its operational target becomes zero.
var ErrWorkerSuspended = diagnostic.NewDiagnosticError("SQL0107", diagnostic.RecoveryPermanent,
	"worker is suspended", "stop this execution; unsuspend the worker to allow a new claim")
