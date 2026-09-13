package datastore

import (
	"context"
	"time"
)

func (d *MetricDatastore) CurrentTime(ctx context.Context) (time.Time, error) {
	var current time.Time
	err := d.DatastoreRetry.WrapIdempotent(ctx, func() error {
		var err error
		current, err = d.currentTime(ctx)
		return err
	})
	return current, err
}

func (d *MetricDatastore) currentTime(ctx context.Context) (time.Time, error) {
	sql := `
		-- sqlstreams: metric.currentTime
		SELECT now();
	`
	var current time.Time
	err := d.Datastore.Pool.QueryRow(ctx, sql).Scan(&current)
	return current, err
}
