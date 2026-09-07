package datastore

import (
	"context"
	"time"
)

func (d *MetricsDatastore) CurrentTime(ctx context.Context) (time.Time, error) {
	var current time.Time
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		current, err = d.currentTime(ctx)
		return err
	})
	return current, err
}

func (d *MetricsDatastore) currentTime(ctx context.Context) (time.Time, error) {
	sql := `
		-- vulkan: metrics.currentTime
		SELECT now();
	`
	var current time.Time
	err := d.Datastore.Pool.QueryRow(ctx, sql).Scan(&current)
	return current, err
}
