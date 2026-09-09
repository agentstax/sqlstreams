// mirrors internal/stream's table-name functions -- the per-stream table name is
// the scope, so every family table interpolates the stream id
export function messageLogTable(streamId: number): string {
	return `message_log_${streamId}`;
}

export function messageLogPartitionTable(streamId: number, n: number): string {
	return `${messageLogTable(streamId)}_${n}`;
}

export function idempotencyKeyTable(streamId: number): string {
	return `idempotency_key_${streamId}`;
}

export function exceptionQueueTable(streamId: number): string {
	return `exception_queue_${streamId}`;
}

export function deliveryLogTable(streamId: number): string {
	return `delivery_log_${streamId}`;
}

export function consumerGroupCursorTable(streamId: number): string {
	return `consumer_group_cursor_${streamId}`;
}

export function claimLeaseTable(streamId: number): string {
	return `claim_lease_${streamId}`;
}

export function messageKeyLeaseTable(streamId: number): string {
	return `message_key_lease_${streamId}`;
}

export function compactionHeadTable(streamId: number): string {
	return `compaction_head_${streamId}`;
}

export function bindingConfigTable(streamId: number): string {
	return `binding_config_${streamId}`;
}

export function bindingConfigLogTable(streamId: number): string {
	return `binding_config_log_${streamId}`;
}
