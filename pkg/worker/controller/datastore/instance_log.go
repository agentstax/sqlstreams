package datastore

import (
	"context"
	"fmt"
	"time"

	"github.com/agentstax/vulkan/pkg/datastore"
)

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
