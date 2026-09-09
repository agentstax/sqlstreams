package stream

import (
	"fmt"
)

// MessageLogTable is streamID's own physical message log.
func MessageLogTable(streamID int64) string {
	return fmt.Sprintf("message_log_%d", streamID)
}

// MessageLogIdSequence is MessageLogTable's id sequence -- the name BIGSERIAL
// gives it.
func MessageLogIdSequence(streamID int64) string {
	return fmt.Sprintf("%s_id_seq", MessageLogTable(streamID))
}

// MessageLogPartitionTable is MessageLogTable's nth partition -- message_log_<stream_id>_<n>.
func MessageLogPartitionTable(streamID, n int64) string {
	return fmt.Sprintf("%s_%d", MessageLogTable(streamID), n)
}

// ExceptionQueueTable is streamID's own physical exception queue -- deliveries
// off the mainline path: ready-to-retry, deferred, dead.
func ExceptionQueueTable(streamID int64) string {
	return fmt.Sprintf("exception_queue_%d", streamID)
}

// DeliveryLogTable is streamID's own physical delivery audit log -- it exists
// for every stream; the stream's delivery_log_mode only gates the writes.
func DeliveryLogTable(streamID int64) string {
	return fmt.Sprintf("delivery_log_%d", streamID)
}

// IdempotencyKeyTable is streamID's own physical idempotency claim table.
func IdempotencyKeyTable(streamID int64) string {
	return fmt.Sprintf("idempotency_key_%d", streamID)
}

// ConsumerGroupCursorTable is streamID's own physical consumer-group cursor table.
func ConsumerGroupCursorTable(streamID int64) string {
	return fmt.Sprintf("consumer_group_cursor_%d", streamID)
}

// ClaimLeaseTable is streamID's own physical claimed-range lease table.
func ClaimLeaseTable(streamID int64) string {
	return fmt.Sprintf("claim_lease_%d", streamID)
}

// MessageKeyLeaseTable is streamID's own physical message-key lease table.
func MessageKeyLeaseTable(streamID int64) string {
	return fmt.Sprintf("message_key_lease_%d", streamID)
}

// CompactionHeadTable is streamID's own physical compaction head index.
func CompactionHeadTable(streamID int64) string {
	return fmt.Sprintf("compaction_head_%d", streamID)
}

// BindingConfigTable is streamID's own physical routing-rule table.
func BindingConfigTable(streamID int64) string {
	return fmt.Sprintf("binding_config_%d", streamID)
}

// BindingConfigLogTable is streamID's own physical binding declaration log.
func BindingConfigLogTable(streamID int64) string {
	return fmt.Sprintf("binding_config_log_%d", streamID)
}
