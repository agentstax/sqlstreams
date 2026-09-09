package datastore

import (
	"context"
	"fmt"
	"time"
)

// The guards: whether the run measured what it claims to.

// ScheduleSlips: seconds in [from, to) in which a produce started more than
// tolerance behind its scheduled instant. Past the tolerance the pacer was
// blocked on its in-flight cap or the process was starved, and the
// generator, not the library, was the limiter for that second.
func (d *CheckerDatastore) CountScheduleSlips(ctx context.Context, from time.Time, to time.Time, tolerance time.Duration) (Measurement, error) {
	slipsSql := fmt.Sprintf(`
		-- lab: datastore.CountScheduleSlips
		SELECT
			count(*),
			COALESCE((array_agg(second::text ORDER BY second))[1:%[2]d], ARRAY[]::text[])
		FROM (
			SELECT DISTINCT date_trunc('second', scheduled_at) AS second
			FROM %[1]s
			WHERE kind = 'attempted' AND scheduled_at >= $1 AND scheduled_at < $2 AND at - scheduled_at > $3
		) AS slipped;
	`, produceRecord, exampleLimit)
	return d.measure(ctx, exampleSecond, slipsSql, from, to, tolerance)
}

// BacklogSlope is the group's backlog trend over [from, to) in messages per
// second, a least-squares fit over the observer's backlog samples; 0 with
// fewer than two samples.
func (d *CheckerDatastore) ReadBacklogSlope(ctx context.Context, from time.Time, to time.Time, streamName string, groupName string) (float64, error) {
	slopeSql := fmt.Sprintf(`
		-- lab: datastore.ReadBacklogSlope
		SELECT COALESCE(regr_slope((highest_message - committed)::double precision, EXTRACT(EPOCH FROM at)), 0)
		FROM %[1]s
		WHERE at >= $1 AND at < $2 AND stream = $3 AND "group" = $4;
	`, observerBacklog)
	var slope float64
	err := d.pool.QueryRow(ctx, slopeSql, from, to, streamName, groupName).Scan(&slope)
	return slope, err
}

// ReadContainerCpu is the median CPU of one compose service's containers
// over [from, to), as a percent of one core; 0 with no samples.
func (d *CheckerDatastore) ReadContainerCpu(ctx context.Context, from time.Time, to time.Time, service string) (float64, error) {
	cpuSql := fmt.Sprintf(`
		-- lab: datastore.ReadContainerCpu
		SELECT COALESCE(percentile_cont(0.5) WITHIN GROUP (ORDER BY cpu_percent), 0)
		FROM %[1]s
		WHERE at >= $1 AND at < $2 AND service = $3;
	`, containerSample)
	var cpu float64
	err := d.pool.QueryRow(ctx, cpuSql, from, to, service).Scan(&cpu)
	return cpu, err
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
