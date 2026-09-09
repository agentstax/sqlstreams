package stream

import (
	"github.com/agentstax/sqlstreams/pkg/common/diagnostic"
)

// ErrStreamConfigMismatch means Register was called with a PartitionSize the stream wasn't created with.
// Every other config field can be changed by registering again.
var ErrStreamConfigMismatch = diagnostic.NewDiagnosticError("SS0004", diagnostic.RecoveryPermanent,
	"stream partition size does not match the existing stream",
	"register with PartitionSize {existing_partition_size}, or use a new stream name")

// ErrStreamNotFound means the named stream has no row.
//
// Diagnose queries: sqlstreams explain SS0005
var ErrStreamNotFound = diagnostic.NewDiagnosticError("SS0005", diagnostic.RecoveryPermanent,
	"stream not found",
	"register it with Client.Stream(name).Register first",

	diagnostic.NewDiagnosticQuery("the stream row under this name", `
SELECT id, name, created_at FROM {schema}.stream_config WHERE name = '{stream}';`),
	diagnostic.NewDiagnosticQuery("the stream row behind an id, if that is what the line carried", `
SELECT id, name, created_at FROM {schema}.stream_config WHERE id = {stream_id};`),
	diagnostic.NewDiagnosticQuery("every registered stream, if the name itself is wrong", `
SELECT name FROM {schema}.stream_config ORDER BY name;`),
)

// ErrStreamNotEmpty means Destroy was called on a stream that still holds
// messages, without an explicit force override.
//
// Diagnose queries: sqlstreams explain SS0006
var ErrStreamNotEmpty = diagnostic.NewDiagnosticError("SS0006", diagnostic.RecoveryPermanent,
	"stream still holds messages",
	"pass DestroyOptions.Force to destroy them with the stream",

	diagnostic.NewDiagnosticQuery("how many messages the destroy would discard", `
SELECT count(*) AS message_count FROM {schema}.message_log_{stream_id};`),
	diagnostic.NewDiagnosticQuery("the newest of them, to judge whether the stream is still in use", `
SELECT
	id,
	routing_key,
	created_at
FROM {schema}.message_log_{stream_id}
ORDER BY id DESC
LIMIT 20;`),
)

// ErrStreamNameTaken means Rename's target name already belongs to another stream.
var ErrStreamNameTaken = diagnostic.NewDiagnosticError("SS0007", diagnostic.RecoveryPermanent,
	"stream name already taken",
	"choose a different name")

// ErrStreamPartitionsRemain means Destroy kept finding new partitions after
// its drop-pass limit -- a producer is likely still writing.
//
// Diagnose queries: sqlstreams explain SS0020
var ErrStreamPartitionsRemain = diagnostic.NewDiagnosticError("SS0020", diagnostic.RecoveryPermanent,
	"stream partitions remain after draining",
	"stop the stream's producers and call Client.Stream(name).Destroy again",

	diagnostic.NewDiagnosticQuery("the partitions still attached to the log", `
SELECT partition.relname AS partition
FROM pg_inherits
JOIN pg_class AS partition ON partition.oid = pg_inherits.inhrelid
WHERE pg_inherits.inhparent = to_regclass('{schema}.message_log_{stream_id}')
ORDER BY partition.relname;`),
	diagnostic.NewDiagnosticQuery("whether a producer is still writing -- run it twice", `
SELECT max(id) AS head, count(*) AS message_count FROM {schema}.message_log_{stream_id};`),
)

// ErrStreamDeclarationInterrupted means the stream row was destroyed between
// the declaration's config write and its re-read; an unchanged retry
// registers the stream fresh, so DatastoreRetry heals the race.
var ErrStreamDeclarationInterrupted = diagnostic.NewDiagnosticError("SS0021", diagnostic.RecoveryTransient,
	"could not finish the stream declaration",
	"run Client.Stream(name).Register again if the stream should still exist")

// ErrDestroyDisabled means a Destroy* call ran without AllowDestroy set
// on the admin's config.
var ErrDestroyDisabled = diagnostic.NewDiagnosticError("SS0008", diagnostic.RecoveryPermanent,
	"destroy is disabled",
	"set ClientConfig.AllowDestroy")

// ErrReservedStreamName means Register/Rename touched a name under
// SystemStreamPrefix -- reserved for the admin's own system streams.
var ErrReservedStreamName = diagnostic.NewDiagnosticError("SS0009", diagnostic.RecoveryPermanent,
	"stream name uses the reserved __system. prefix",
	"choose a name outside the __system. prefix")
