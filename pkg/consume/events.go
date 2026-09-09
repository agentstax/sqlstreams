package consume

import (
	"github.com/agentstax/sqlstreams/pkg/common/diagnostic"
)

// EventLeaseReclaimed means a range lease's worker stopped renewing and the
// range went back to the claimable pool.
//
// Diagnose queries: sqlstreams explain SS0026
var EventLeaseReclaimed = diagnostic.NewDiagnosticEvent("SS0026",
	"lease reclaimed from expired worker", "",

	diagnostic.NewDiagnosticQuery("the leases this group holds now", `
SELECT
	token,
	low,
	high,
	expires_at,
	reclaims
FROM {schema}.claim_lease_{stream_id}
WHERE consumer_group_id = {group_id}
ORDER BY low;`),
	diagnostic.NewDiagnosticQuery("what the reclaimed range left behind", `
SELECT
	message_id,
	status,
	attempts,
	last_error
FROM {schema}.exception_queue_{stream_id}
WHERE consumer_group_id = {group_id}
	AND message_id BETWEEN {low} AND {high}
ORDER BY message_id;`),
)

// EventRangeQuarantined means a range hit MaxRangeReclaims and is treated as
// poison instead of being handed out again.
//
// Diagnose queries: sqlstreams explain SS0027
var EventRangeQuarantined = diagnostic.NewDiagnosticEvent("SS0027",
	"range quarantined after max reclaims",
	"messages written as 'ready' exceptions",

	diagnostic.NewDiagnosticQuery("the exceptions the quarantine wrote", `
SELECT
	message_id,
	status,
	attempts,
	can_run_after,
	last_error
FROM {schema}.exception_queue_{stream_id}
WHERE consumer_group_id = {group_id}
	AND message_id BETWEEN {low} AND {high}
ORDER BY message_id;`),
	diagnostic.NewDiagnosticQuery("the messages in the range, to find what kills a consumer", `
SELECT
	id,
	routing_key,
	payload
FROM {schema}.message_log_{stream_id}
WHERE id BETWEEN {low} AND {high}
ORDER BY id;`),
)

// EventMessagesDeadLettered marks a commit that wrote terminal outcomes for
// a batch of messages.
//
// Diagnose queries: sqlstreams explain SS0028
var EventMessagesDeadLettered = diagnostic.NewDiagnosticEvent("SS0028",
	"messages dead-lettered",
	"unrecoverable, will not be retried",

	diagnostic.NewDiagnosticQuery("every dead row this group holds, newest first", `
SELECT
	message_id,
	attempts,
	last_error,
	updated_at
FROM {schema}.exception_queue_{stream_id}
WHERE consumer_group_id = {group_id}
	AND status = 'dead'
ORDER BY updated_at DESC;`),
	diagnostic.NewDiagnosticQuery("which errors account for them", `
SELECT last_error, count(*) AS dead_count
FROM {schema}.exception_queue_{stream_id}
WHERE consumer_group_id = {group_id}
	AND status = 'dead'
GROUP BY last_error
ORDER BY dead_count DESC;`),
)

// EventMessageDeadLettered marks one delivery written as terminal.
//
// Diagnose queries: sqlstreams explain SS0029
var EventMessageDeadLettered = diagnostic.NewDiagnosticEvent("SS0029",
	"message dead-lettered",
	"unrecoverable, will not be retried",

	diagnostic.NewDiagnosticQuery("the delivery row the dead-lettering wrote", `
SELECT
	status,
	attempts,
	last_error,
	updated_at
FROM {schema}.exception_queue_{stream_id}
WHERE consumer_group_id = {group_id}
	AND message_id = {message_id};`),
	diagnostic.NewDiagnosticQuery("every attempt it made, oldest first", `
SELECT
	attempt,
	status,
	error,
	attempted_at
FROM {schema}.delivery_log_{stream_id}
WHERE consumer_group_id = {group_id}
	AND message_id = {message_id}
ORDER BY attempt;`),
	diagnostic.NewDiagnosticQuery("the message itself", `
SELECT
	id,
	routing_key,
	payload,
	created_at
FROM {schema}.message_log_{stream_id}
WHERE id = {message_id};`),
)

// EventExceptionDeadLettered marks one exception written as terminal.
//
// Diagnose queries: sqlstreams explain SS0030
var EventExceptionDeadLettered = diagnostic.NewDiagnosticEvent("SS0030",
	"exception dead-lettered",
	"unrecoverable, will not be retried",

	diagnostic.NewDiagnosticQuery("the exception row now recorded dead", `
SELECT
	status,
	attempts,
	last_error,
	updated_at
FROM {schema}.exception_queue_{stream_id}
WHERE consumer_group_id = {group_id}
	AND message_id = {message_id};`),
	diagnostic.NewDiagnosticQuery("the attempts that exhausted its budget", `
SELECT
	attempt,
	status,
	error,
	attempted_at
FROM {schema}.delivery_log_{stream_id}
WHERE consumer_group_id = {group_id}
	AND message_id = {message_id}
ORDER BY attempt;`),
)

// EventKillBackstopFired means the crash-loop backstop marked a group's
// exceptions dead after repeated consumer crashes on the same rows.
//
// Diagnose queries: sqlstreams explain SS0031
var EventKillBackstopFired = diagnostic.NewDiagnosticEvent("SS0031",
	"crash-loop kill backstop fired",
	"exceptions marked dead",

	diagnostic.NewDiagnosticQuery("the rows the backstop marked dead", `
SELECT
	message_id,
	attempts,
	last_error,
	updated_at
FROM {schema}.exception_queue_{stream_id}
WHERE consumer_group_id = {group_id}
	AND status = 'dead'
ORDER BY updated_at DESC;`),
	diagnostic.NewDiagnosticQuery("the attempts that crashed without recording an outcome", `
SELECT
	message_id,
	attempt,
	status,
	attempted_at
FROM {schema}.delivery_log_{stream_id}
WHERE consumer_group_id = {group_id}
	AND status = 'expired'
ORDER BY attempted_at DESC
LIMIT 50;`),
)

// EventStoredOptionsClamped means a stored message's options fell outside
// this consumer's MessageMin/MessageMax bounds.
var EventStoredOptionsClamped = diagnostic.NewDiagnosticEvent("SS0032",
	"stored message options outside this consumer's bounds", "clamped")

// EventSlowDispatch means one delivery's dispatch ran past the group's
// SlowDispatchThreshold, whatever the delivery's outcome.
var EventSlowDispatch = diagnostic.NewDiagnosticEvent("SS0039",
	"delivery dispatch exceeded the duration threshold", "")

// EventGroupConfigNotRefreshed means an instance could not read its worker
// row's stored config back, so it keeps running on the copy it already has.
//
// Diagnose queries: sqlstreams explain SS0060
var EventGroupConfigNotRefreshed = diagnostic.NewDiagnosticEvent("SS0060",
	"could not refresh group config",
	"the last copy stays in use",

	diagnostic.NewDiagnosticQuery("the config document stored on this group's worker rows", `
SELECT worker_config.name, worker_config.metadata
FROM {schema}.worker_config
JOIN {schema}.consumer_group_config ON consumer_group_config.id = worker_config.consumer_group_id
WHERE consumer_group_config.name = '{group}'
ORDER BY worker_config.name;`),
)

// EventConsumerStopped is the session summary a consumer instance logs on
// every exit.
var EventConsumerStopped = diagnostic.NewDiagnosticEvent("SS0041",
	"consumer stopped", "")

// EventQueuedRangeStale means a queued message cannot start within its range's lease budget.
var EventQueuedRangeStale = diagnostic.NewDiagnosticEvent("SS0105",
	"queued message has insufficient lease time",
	"range waits for reclaim; successful handlers in the range can repeat")
