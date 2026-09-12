package datastore

import (
	"encoding/json"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
)

// DeliveryStatus is the exception_queue_<stream_id>.status column's value set. The
// lifecycle path moves 'ready' -> 'processing' -> 'done'/'dead'; the
// exception window writes 'ready'/'deferred', leases 'inflight', kills 'dead'.
type DeliveryStatus string

const (
	DeliveryReady      DeliveryStatus = "ready"
	DeliveryProcessing DeliveryStatus = "processing"
	DeliveryInflight   DeliveryStatus = "inflight"
	DeliveryDeferred   DeliveryStatus = "deferred"
	DeliveryDone       DeliveryStatus = "done"
	DeliveryDead       DeliveryStatus = "dead"
)

// ExceptionQueueRow is one (consumer_group_id, message_id) row of the per-stream
// exception_queue_<stream_id> table: the mutable per-consumer lifecycle state that
// lives off the immutable message_log. Payload is not stored on the row --
// it's joined back in from message_log at claim time. The lifecycle claim
// path never sets the lease columns (lease_expires_at / lease_token) -- no crash
// recovery there; the exception window is what leases through them.
type ExceptionQueueRow struct {
	ConsumerGroupId int64                  `db:"consumer_group_id"`
	StreamId        int64                  `db:"stream_id"`
	MessageId       int64                  `db:"message_id"`
	Payload         json.RawMessage        `db:"payload"`
	Status          DeliveryStatus         `db:"status"`
	Attempts        int                    `db:"attempts"`
	Options         *common.MessageOptions `db:"options"`
}
