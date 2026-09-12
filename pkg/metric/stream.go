package metric

import (
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
)

// MetricStreamName is __system.metrics
const MetricStreamName = common.SystemStreamPrefix + "metrics"

// StreamSnapshot is a stream's live metrics, read from its tables at the
// moment of the call, with every consumer group's snapshot beside it.
type StreamSnapshot struct {
	StreamId                          int64                   `json:"stream_id"`
	Partitions                        int64                   `json:"partitions"`
	Compacted                         bool                    `json:"compacted"`                              // any compaction_head row points at a message
	CompactionRowsWithoutHead         int64                   `json:"compaction_rows_without_head"`           // keys whose head row no longer points at a message
	OldestCompactionRowWithoutHeadAge time.Duration           `json:"oldest_compaction_row_without_head_age"` // 0 when there are none
	Groups                            []ConsumerGroupSnapshot `json:"groups"`
}

// StreamSchemaVersionSnapshot is one payload version's presence in a stream's log.
type StreamSchemaVersionSnapshot struct {
	Version         int                             `json:"version"`
	Messages        int64                           `json:"messages"`         // rows in the log at this version
	CompactionHeads int64                           `json:"compaction_heads"` // keys whose current head is at this version
	Groups          []ConsumerGroupSchemaVersionLag `json:"groups"`
}

// ConsumerGroupSchemaVersionLag is one consumer group's unread and unresolved rows
// at one payload version.
type ConsumerGroupSchemaVersionLag struct {
	ConsumerGroup        string `json:"group"`
	Unconsumed           int64  `json:"unconsumed"`            // rows at the version above the group's committed cursor
	UnresolvedExceptions int64  `json:"unresolved_exceptions"` // delivery rows at the version still 'ready', 'inflight', or 'deferred'
}
