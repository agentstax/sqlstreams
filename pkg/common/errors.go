package common

import "github.com/agentstax/sqlstreams/pkg/common/diagnostic"

// ErrAlreadyConsuming means Consume ran twice at once on one instance -- an
// instance runs one Consume at a time.
var ErrAlreadyConsuming = diagnostic.NewDiagnosticError("SS0001", diagnostic.RecoveryPermanent,
	"instance is already consuming",
	"wait for the running Consume to return, or Register another instance")

// ErrLifecycleContextNotCancellable means Consume's ctx can never be
// cancelled (e.g. context.Background()), so shutdown could never be
// requested.
var ErrLifecycleContextNotCancellable = diagnostic.NewDiagnosticError("SS0002", diagnostic.RecoveryPermanent,
	"lifecycle context can never be cancelled",
	"pass the application's shutdown context, or set ConsumeOptions.DisableGracefulShutdown")

// ErrLeaseLost means the row was reclaimed by another consumer between the
// claim and the write; the delivery machinery handles the redelivery.
var ErrLeaseLost = diagnostic.NewDiagnosticError("SS0003", diagnostic.RecoveryPermanent,
	"lease lost to another consumer", "")

// ErrCommitConfirmationLost means the connection died at Commit with
// outcomes already queued: whether they landed is unconfirmable, so a retry
// could record duplicates -- the lease's expiry sorts the truth out.
//
// Diagnose queries: sqlstreams explain SS0019
var ErrCommitConfirmationLost = diagnostic.NewDiagnosticError("SS0019", diagnostic.RecoveryPermanent,
	"commit confirmation was lost", "",

	diagnostic.NewDiagnosticQuery("whether the outcomes landed -- rows updated at the commit", `
SELECT
	message_id,
	status,
	attempts,
	updated_at
FROM {schema}.exception_queue_{stream_id}
WHERE consumer_group_id = {group_id}
ORDER BY updated_at DESC
LIMIT 20;`),
	diagnostic.NewDiagnosticQuery("the range lease whose expiry settles it either way", `
SELECT
	token,
	low,
	high,
	expires_at,
	reclaims
FROM {schema}.claim_lease_{stream_id}
WHERE consumer_group_id = {group_id};`),
)

// ErrPayloadNotEncodable means encoding/json rejected the payload -- a NaN
// float, a channel or func field, a MarshalJSON that returned an error. The
// payload is encoded in Go before it reaches pgx because pgx's own encode
// failure prints the value it could not encode.
var ErrPayloadNotEncodable = diagnostic.NewDiagnosticError("SS0097", diagnostic.RecoveryPermanent,
	"payload cannot be encoded as JSON",
	"give the payload a shape encoding/json accepts -- the cause names what it could not encode")
