package datastore

import (
	"context"
	"errors"
	"fmt"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/worker"
	"github.com/jackc/pgx/v5"
)

func (d *WorkerDatastore) UpdateTargetInstances(ctx context.Context, workerId int64, target int) error {
	return d.DatastoreRetry.WrapIdempotent(ctx, func() error {
		return d.updateTargetInstances(ctx, workerId, target)
	})
}

func (d *WorkerDatastore) updateTargetInstances(ctx context.Context, workerId int64, target int) error {
	tx, err := d.Datastore.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	readSql := fmt.Sprintf(`
		-- sqlstreams: worker.updateTargetInstances
		SELECT target_instances
		FROM %[1]s.worker_config
		WHERE id = $1
		FOR UPDATE;
	`, d.Datastore.Schema)
	var previous int
	err = tx.QueryRow(ctx, readSql, workerId).Scan(&previous)
	if errors.Is(err, pgx.ErrNoRows) {
		return worker.ErrWorkerNotFound.With("worker_id", workerId)
	}
	if err != nil {
		return err
	}
	if previous == target {
		return tx.Commit(ctx)
	}

	updateSql := fmt.Sprintf(`
		-- sqlstreams: worker.updateTargetInstances
		UPDATE %[1]s.worker_config
		SET target_instances = $2, updated_at = now()
		WHERE id = $1;
	`, d.Datastore.Schema)
	if _, err := tx.Exec(ctx, updateSql, workerId, target); err != nil {
		return err
	}
	if err := d.appendWorkerConfigLog(ctx, tx, workerId, common.ProcessIdentity); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	d.Logger.InfoContext(ctx, "worker target updated", "worker_id", workerId, "target_instances", target)
	return nil
}
