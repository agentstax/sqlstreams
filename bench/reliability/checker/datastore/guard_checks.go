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

// BacklogSlope is the group's backlog trend over [from, to) in messages per
// second, a least-squares fit over the observer's backlog samples; 0 with
// fewer than two samples.
func (d *CheckerDatastore) ReadBacklogSlope(ctx context.Context, from time.Time, to time.Time, topicName string, groupName string) (float64, error) {
	slopeSql := fmt.Sprintf(`
		-- lab: datastore.ReadBacklogSlope
		SELECT COALESCE(regr_slope((highest_message - committed)::double precision, EXTRACT(EPOCH FROM at)), 0)
		FROM %[1]s
		WHERE at >= $1 AND at < $2 AND topic = $3 AND "group" = $4;
	`, observerBacklog)
	var slope float64
	err := d.pool.QueryRow(ctx, slopeSql, from, to, topicName, groupName).Scan(&slope)
	return slope, err
}

// HeadroomBreaches: container samples in [from, to) where the producer or
// consumer service sat above the given fraction of its CPU cap; a container
// compose left uncapped is judged against uncappedCpus, the engine's own
// count.
func (d *CheckerDatastore) CountHeadroomBreaches(ctx context.Context, from time.Time, to time.Time, fraction float64, uncappedCpus float64) (Measurement, error) {
	breachesSql := fmt.Sprintf(`
		-- lab: datastore.CountHeadroomBreaches
		SELECT
			count(*),
			COALESCE((array_agg(to_char(at, 'HH24:MI:SS') || ' ' || service || ' ' || round(cpu_percent) || '%%' ORDER BY at))[1:%[2]d], ARRAY[]::text[])
		FROM %[1]s
		WHERE at >= $1 AND at < $2
			AND service IN ('producer', 'consumer')
			AND cpu_percent > $3::double precision * 100 * COALESCE(NULLIF(cpus, 0), $4::double precision);
	`, containerSample, exampleLimit)
	return d.measure(ctx, exampleSample, breachesSql, from, to, fraction, uncappedCpus)
}
