package datastore

import (
	"context"
	"fmt"
	"time"
)

// The guards: whether the run measured what it claims to.

// ScheduleSlips: seconds in which a produce started more than tolerance
// behind its scheduled instant. Past the tolerance the pacer was blocked on
// its in-flight cap or the process was starved, and the generator, not the
// library, was the limiter for that second.
func (d *CheckerDatastore) CountScheduleSlips(ctx context.Context, tolerance time.Duration) (Measurement, error) {
	slipsSql := fmt.Sprintf(`
		-- lab: datastore.CountScheduleSlips
		SELECT
			count(*),
			COALESCE((array_agg(second::text ORDER BY second))[1:%[2]d], ARRAY[]::text[])
		FROM (
			SELECT DISTINCT date_trunc('second', scheduled_at) AS second
			FROM %[1]s
			WHERE kind = 'attempted' AND at - scheduled_at > $1
		) AS slipped;
	`, produceRecord, exampleLimit)
	return d.measure(ctx, exampleSecond, slipsSql, tolerance)
}
