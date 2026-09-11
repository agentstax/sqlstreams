package datastore

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	sqlstreamsdatastore "github.com/agentstax/sqlstreams/pkg/datastore"
)

type RunnerDatastore struct{ pool *pgxpool.Pool }

func NewRunnerDatastore(pool *pgxpool.Pool) (*RunnerDatastore, error) {
	if pool == nil {
		return nil, errors.New("pool must not be nil")
	}
	return &RunnerDatastore{pool: pool}, nil
}

// SuspendExceptionConsumers disables exception workers before measured production.
func (d *RunnerDatastore) SuspendExceptionConsumers(ctx context.Context) error {
	suspendSql := fmt.Sprintf(`
        -- lab: datastore.SuspendExceptionConsumers
        UPDATE %[1]s.worker_config
        SET target_instances = 0, updated_at = now()
        WHERE name = 'exception_consumer' AND target_instances <> 0;
    `, sqlstreamsdatastore.DefaultSchema)
	_, err := d.pool.Exec(ctx, suspendSql)
	return err
}

func (d *RunnerDatastore) ExceptionConsumersStopped(ctx context.Context) (bool, error) {
	stoppedSql := fmt.Sprintf(`
        -- lab: datastore.ExceptionConsumersStopped
        SELECT NOT EXISTS (
            SELECT 1 FROM %[1]s.worker_config w
            WHERE w.name = 'exception_consumer' AND (
                w.target_instances <> 0 OR EXISTS (
                    SELECT 1 FROM %[1]s.worker_instance i WHERE i.worker_id = w.id
                )
            )
        );
    `, sqlstreamsdatastore.DefaultSchema)
	var stopped bool
	err := d.pool.QueryRow(ctx, stoppedSql).Scan(&stopped)
	return stopped, err
}
