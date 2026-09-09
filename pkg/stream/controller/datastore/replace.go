package datastore

import (
	"context"
	"fmt"
	"time"

	"github.com/agentstax/sqlstreams/pkg/stream"
)

// replaceConfig overwrites an already-registered stream's mutable config
// with declared's -- the newest declaration wins -- and appends the new
// snapshot to stream_config_log in the same transaction.
// partition_size is not mutable config.
func (d *StreamDatastore) replaceConfig(ctx context.Context, found *StreamConfigRow, declared *StreamConfigRow, declaredBy string) (*StreamConfigRow, error) {
	if found.PartitionSize != declared.PartitionSize {
		return nil, stream.ErrStreamConfigMismatch.With(
			"stream", found.Name,
			"existing_partition_size", found.PartitionSize, "declared_partition_size", declared.PartitionSize)
	}

	changes := configChanges(found, declared)
	if len(changes) == 0 {
		d.Logger.InfoContext(ctx, "stream registered (already existed)", "stream", found.Name, "stream_id", found.Id)
		return found, nil
	}

	tx, err := d.Datastore.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	sql := fmt.Sprintf(`
		-- sqlstreams: stream.replaceConfig
		UPDATE %[1]s.stream_config
		SET
			retention_ttl_ns = $2,
			allow_drop_past_committed = $3,
			idempotency_key_ttl_ns = $4,
			empty_compaction_head_ttl_ns = $5,
			delivery_log_mode = $6,
			updated_at = NOW()
		WHERE id = $1
		RETURNING
			id,
			system_id,
			name,
			partition_size,
			retention_ttl_ns,
			allow_drop_past_committed,
			idempotency_key_ttl_ns,
			empty_compaction_head_ttl_ns,
			delivery_log_mode,
			created_at,
			updated_at;
	`, d.Datastore.Schema)
	row := tx.QueryRow(ctx, sql,
		found.Id,
		declared.RetentionTTLNs,
		declared.AllowDropPastCommitted,
		declared.IdempotencyKeyTTLNs,
		declared.EmptyCompactionHeadTTLNs,
		declared.DeliveryLogMode,
	)
	updated, err := d.scanStreamConfigRow(row)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, stream.ErrStreamDeclarationInterrupted.With("stream", found.Name)
	}

	if err := d.appendStreamConfigLog(ctx, tx, updated, declaredBy); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	d.Logger.WarnContext(ctx, stream.EventStreamConfigReplaced.Message(),
		append([]any{"code", stream.EventStreamConfigReplaced.GetCode(), "stream", updated.Name, "stream_id", updated.Id}, changes...)...)
	return updated, nil
}

// ***************
// *** HELPERS ***
// ***************

// configChanges is every mutable config field the declaration would change,
// as log args. Empty means the declaration matches what is stored.
func configChanges(found *StreamConfigRow, declared *StreamConfigRow) []any {
	var changes []any
	if found.RetentionTTLNs != declared.RetentionTTLNs {
		changes = append(changes, "retention_ttl", replaced(time.Duration(found.RetentionTTLNs), time.Duration(declared.RetentionTTLNs)))
	}
	if found.AllowDropPastCommitted != declared.AllowDropPastCommitted {
		changes = append(changes, "allow_drop_past_committed", replaced(found.AllowDropPastCommitted, declared.AllowDropPastCommitted))
	}
	if found.IdempotencyKeyTTLNs != declared.IdempotencyKeyTTLNs {
		changes = append(changes, "idempotency_key_ttl", replaced(time.Duration(found.IdempotencyKeyTTLNs), time.Duration(declared.IdempotencyKeyTTLNs)))
	}
	if found.EmptyCompactionHeadTTLNs != declared.EmptyCompactionHeadTTLNs {
		changes = append(changes, "empty_compaction_head_ttl", replaced(time.Duration(found.EmptyCompactionHeadTTLNs), time.Duration(declared.EmptyCompactionHeadTTLNs)))
	}
	if found.DeliveryLogMode != declared.DeliveryLogMode {
		changes = append(changes, "delivery_log_mode", replaced(found.DeliveryLogMode, declared.DeliveryLogMode))
	}
	return changes
}

// replaced renders one field's change as the log line carries it: old -> new.
func replaced(stored any, declared any) string {
	return fmt.Sprintf("%v -> %v", stored, declared)
}
