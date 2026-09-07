package datastore

import (
	"context"
	"fmt"
	"time"

	"github.com/agentstax/vulkan/pkg/datastore"
)

func (d *WorkerDatastore) ListInstanceSnapshots(ctx context.Context, workerId int64, start time.Time, end time.Time) ([]WorkerInstanceSnapshotRow, error) {
	var snapshots []WorkerInstanceSnapshotRow
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		snapshots, err = d.listInstanceSnapshots(ctx, workerId, start, end)
		return err
	})
	return snapshots, err
}

func (d *WorkerDatastore) listInstanceSnapshots(ctx context.Context, workerId int64, start time.Time, end time.Time) ([]WorkerInstanceSnapshotRow, error) {
	sql := fmt.Sprintf(`
		-- vulkan: worker.listInstanceSnapshots
		SELECT id, worker_instance_id, worker_id, token, expires_at, attempts, created_at, attempted_at
		FROM %[1]s.worker_instance_log
		WHERE worker_id = $1
			AND expires_at >= $2
			AND created_at <= $3
			AND attempted_at <= $3
		ORDER BY created_at DESC, expires_at DESC, id DESC;
	`, d.Datastore.Schema)
	rows, err := d.Datastore.Pool.Query(ctx, sql, workerId, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []WorkerInstanceSnapshotRow
	for rows.Next() {
		var snapshot WorkerInstanceSnapshotRow
		if err := rows.Scan(&snapshot.Id, &snapshot.WorkerInstanceId, &snapshot.WorkerId, &snapshot.Token, &snapshot.ExpiresAt, &snapshot.Attempts, &snapshot.CreatedAt, &snapshot.AttemptedAt); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, rows.Err()
}

func (d *WorkerDatastore) SweepExpiredInstanceLogs(ctx context.Context, ttl time.Duration) (int64, error) {
	var removed int64
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		removed, err = d.sweepExpiredInstanceLogs(ctx, ttl)
		return err
	})
	return removed, err
}

func (d *WorkerDatastore) sweepExpiredInstanceLogs(ctx context.Context, ttl time.Duration) (int64, error) {
	sql := fmt.Sprintf(`
		-- vulkan: worker.sweepExpiredInstanceLogs
		DELETE FROM %[1]s.worker_instance_log
		WHERE expires_at < now() - make_interval(secs => $1);
	`, d.Datastore.Schema)
	tag, err := d.Datastore.Pool.Exec(ctx, sql, ttl.Seconds())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// appendWorkerInstanceLog copies the instance inside the transaction that claimed or renewed it.
func (d *WorkerDatastore) appendWorkerInstanceLog(ctx context.Context, q datastore.Querier, instanceId int64) error {
	sql := fmt.Sprintf(`
		-- vulkan: worker.appendWorkerInstanceLog
		INSERT INTO %[1]s.worker_instance_log (worker_instance_id, worker_id, token, expires_at, attempts, created_at)
		SELECT
			id,
			worker_id,
			token,
			expires_at,
			attempts,
			created_at
		FROM %[1]s.worker_instance
		WHERE id = $1;
	`, d.Datastore.Schema)
	_, err := q.Exec(ctx, sql, instanceId)
	return err
}
