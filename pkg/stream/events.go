package stream

import (
	"github.com/agentstax/sqlstreams/pkg/common/diagnostic"
)

// EventStreamConfigReplaced means a declaration overwrote a stream row's
// differing mutable config -- two declarers disagree about the stream.
//
// Diagnose queries: sqlstreams explain SQL0061
var EventStreamConfigReplaced = diagnostic.NewDiagnosticEvent("SQL0061",
	"stream config replaced",
	"the newest declaration wins; if this is unexpected or repeats on every restart, two services declare this stream with different configs and overwrite each other",

	diagnostic.NewDiagnosticQuery("every declaration this stream has received, newest first", `
SELECT
	name,
	retention_ttl_ns,
	allow_drop_past_committed,
	idempotency_key_ttl_ns,
	empty_compaction_head_ttl_ns,
	delivery_log_mode,
	declared_by,
	declared_at
FROM {schema}.stream_config_log
WHERE stream_id = {stream_id}
ORDER BY id DESC
LIMIT 10;`),
)
