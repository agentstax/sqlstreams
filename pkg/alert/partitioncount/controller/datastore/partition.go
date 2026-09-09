package datastore

import (
	"context"
)

// locksPerPartition: a partition owns 5 lockable relations -- table, pkey
// index, message_key index, TOAST table, TOAST index -- and dropping the
// parent locks them all at once.
const locksPerPartition = 5

// PartitionLockCeiling is the partition count at which a DROP/Destroy risks
// "out of shared memory".
func (d *PartitionCountDatastore) PartitionLockCeiling(ctx context.Context) (int64, error) {
	var ceiling int64
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		ceiling, err = d.partitionLockCeiling(ctx)
		return err
	})
	return ceiling, err
}

func (d *PartitionCountDatastore) partitionLockCeiling(ctx context.Context) (int64, error) {
	// the product is the lock table's total size, fixed at server start
	sql := `
		-- sqlstreams: partitioncount.partitionLockCeiling
		SELECT current_setting('max_locks_per_transaction')::bigint
			* (current_setting('max_connections')::bigint
				+ current_setting('max_prepared_transactions')::bigint);
	`
	var size int64
	if err := d.Datastore.Pool.QueryRow(ctx, sql).Scan(&size); err != nil {
		return 0, err
	}
	return size / locksPerPartition, nil
}
