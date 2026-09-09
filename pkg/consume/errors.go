package consume

import (
	"github.com/agentstax/sqlstreams/pkg/common/diagnostic"
)

// ErrConsumerNotFound means the named group has no row on that stream.
//
// Diagnose queries: sqlstreams explain SQL0014
var ErrConsumerNotFound = diagnostic.NewDiagnosticError("SQL0014", diagnostic.RecoveryPermanent,
	"consumer group not found",
	"register a consumer with this group name to create it",

	diagnostic.NewDiagnosticQuery("every group registered on this stream", `
SELECT
	consumer_group_config.id,
	consumer_group_config.name,
	consumer_group_config.created_at
FROM {schema}.consumer_group_config
JOIN {schema}.stream_config ON stream_config.id = consumer_group_config.stream_id
WHERE stream_config.name = '{stream}'
ORDER BY consumer_group_config.name;`),
	diagnostic.NewDiagnosticQuery("the group row behind an id, if that is what the line carried", `
SELECT
	id,
	stream_id,
	name,
	created_at
FROM {schema}.consumer_group_config
WHERE id = {group_id};`),
)

// ErrConsumerGroupLive means Destroy was called while a worker instance still runs
// on the group, without a force override.
//
// Diagnose queries: sqlstreams explain SQL0015
var ErrConsumerGroupLive = diagnostic.NewDiagnosticError("SQL0015", diagnostic.RecoveryPermanent,
	"consumer group still has a live consumer",
	"stop the group's consumers, or pass DestroyOptions.Force",

	diagnostic.NewDiagnosticQuery("the instances still heartbeating on this group", `
SELECT
	worker_config.name AS worker,
	worker_instance.id,
	worker_instance.expires_at,
	worker_instance.attempts
FROM {schema}.worker_instance
JOIN {schema}.worker_config ON worker_config.id = worker_instance.worker_id
WHERE worker_config.consumer_group_id = {group_id}
	AND worker_instance.expires_at > now()
ORDER BY worker_instance.expires_at;`),
)

// ErrConsumerGroupDeliveriesPending means Destroy was called while the group still
// holds delivery rows, without a force override. Deleting them discards:
//   - ready/inflight/deferred rows -> failures promised a retry
//   - dead rows                    -> the dead-letter record
//
// Diagnose queries: sqlstreams explain SQL0016
var ErrConsumerGroupDeliveriesPending = diagnostic.NewDiagnosticError("SQL0016", diagnostic.RecoveryPermanent,
	"consumer group still has delivery rows",
	"pass DestroyOptions.Force to delete them",

	diagnostic.NewDiagnosticQuery("what the delivery rows would discard, by status", `
SELECT status, count(*) AS row_count
FROM {schema}.exception_queue_{stream_id}
WHERE consumer_group_id = {group_id}
GROUP BY status;`),
	diagnostic.NewDiagnosticQuery("the dead ones, whose dead-letter record goes with them", `
SELECT
	message_id,
	attempts,
	last_error,
	updated_at
FROM {schema}.exception_queue_{stream_id}
WHERE consumer_group_id = {group_id}
	AND status = 'dead'
ORDER BY message_id;`),
)

// ErrDeliveryTerminal is what Terminal returns: the handler declared that no
// retry could succeed, so the delivery dead-letters on this attempt.
var ErrDeliveryTerminal = diagnostic.NewDiagnosticError("SQL0055", diagnostic.RecoveryPermanent,
	"delivery cannot succeed",
	"")

// ErrDeliveryDelayed is what Delay returns: the handler asked for a later
// run, so the delivery waits out the delay and no failure is counted.
var ErrDeliveryDelayed = diagnostic.NewDiagnosticError("SQL0054", diagnostic.RecoveryTransient,
	"could not complete the delivery yet, the handler asked to run it later",
	"")
