package datastore

import (
	"context"
	"fmt"
)

// ScheduleSnapshots is every schedule (config row joined to its cursor) with
// its target stream and schedule state.
func (d *MetricDatastore) ScheduleSnapshots(ctx context.Context) ([]ScheduleSnapshotRow, error) {
	var schedules []ScheduleSnapshotRow
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		schedules, err = d.scheduleSnapshots(ctx)
		return err
	})
	return schedules, err
}

func (d *MetricDatastore) scheduleSnapshots(ctx context.Context) ([]ScheduleSnapshotRow, error) {
	sql := fmt.Sprintf(`
		-- sqlstreams: metric.scheduleSnapshots
		SELECT
			j.name,
			j.system_id,
			j.stream_id,
			t.name AS stream_name,
			j.expression,
			j.suspended,
			c.next_scheduled_at,
			c.last_scheduled_at,
			EXTRACT(EPOCH FROM (now() - c.next_scheduled_at)) AS due_for_secs
		FROM %[1]s.schedule_config j
		JOIN %[1]s.schedule_cursor c ON c.schedule_id = j.id
		JOIN %[1]s.stream_config t ON t.id = j.stream_id
		ORDER BY j.name;
	`, d.Datastore.Schema)
	rows, err := d.Datastore.Pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schedules []ScheduleSnapshotRow
	for rows.Next() {
		var data ScheduleSnapshotRow
		if err := rows.Scan(&data.Name, &data.SystemId, &data.StreamId, &data.StreamName,
			&data.Expression, &data.Suspended, &data.NextScheduledAt, &data.LastScheduledAt, &data.DueForSecs); err != nil {
			return nil, err
		}
		schedules = append(schedules, data)
	}
	return schedules, rows.Err()
}
