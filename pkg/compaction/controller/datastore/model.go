package datastore

import (
	"encoding/json"
	"time"
)

// CompactionHeadRow models a compaction_head_<topic_id> table row exactly.
// The three head fields are nil together when the key has a lockable row but
// no current head.
type CompactionHeadRow struct {
	CompactionKey  string    `db:"compaction_key"`
	HeadId         *int64    `db:"head_id"`
	SchemaVersion  *int64    `db:"schema_version"`
	CompactionRank *int64    `db:"compaction_rank"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

// MessageLogRow is one compacted message row.
type MessageLogRow struct {
	Id             int64           `db:"id"`
	Payload        json.RawMessage `db:"payload"`
	CreatedAt      time.Time       `db:"created_at"`
	RoutingKey     string          `db:"routing_key"` // "" if unset, COALESCE'd at read
	MessageKey     string          `db:"message_key"`
	CompactionRank int64           `db:"compaction_rank"`
}
