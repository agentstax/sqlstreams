package datastore

import (
	"context"
	"fmt"
	"time"
)

// LatencySummary is one latency distribution: how many produces it covers
// and the percentiles the server computed over their record rows -- never
// merged from per-process summaries.
type LatencySummary struct {
	Count int64         `json:"count"`
	P50   time.Duration `json:"p50_ns"`
	P90   time.Duration `json:"p90_ns"`
	P99   time.Duration `json:"p99_ns"`
	P999  time.Duration `json:"p999_ns"`
	Max   time.Duration `json:"max_ns"`
}

// ThroughputSample is one second of the run and the produces whose reply
// landed inside it. Seconds with none are present at zero.
type ThroughputSample struct {
	At        time.Time `json:"at"`
	Committed int64     `json:"committed"`
}

// ReadProduceLatency is the produce call's own latency, reply time against
// scheduled time, for the produces scheduled in [from, to).
func (d *CheckerDatastore) ReadProduceLatency(ctx context.Context, from time.Time, to time.Time) (LatencySummary, error) {
	latencySql := fmt.Sprintf(`
		-- lab: datastore.ReadProduceLatency
		SELECT
			count(*),
			COALESCE(percentile_cont(0.5) WITHIN GROUP (ORDER BY latency_seconds), 0),
			COALESCE(percentile_cont(0.9) WITHIN GROUP (ORDER BY latency_seconds), 0),
			COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY latency_seconds), 0),
			COALESCE(percentile_cont(0.999) WITHIN GROUP (ORDER BY latency_seconds), 0),
			COALESCE(max(latency_seconds), 0)
		FROM (
			SELECT EXTRACT(EPOCH FROM (at - scheduled_at))::double precision AS latency_seconds
			FROM %[1]s
			WHERE kind = 'committed' AND scheduled_at >= $1 AND scheduled_at < $2
		) AS latencies;
	`, produceRecord)
	return d.readLatency(ctx, latencySql, from, to)
}

// ReadEndToEndLatency is the handler's first success against scheduled time,
// for the produces scheduled in [from, to) -- what a message waited from the
// moment it was meant to exist until it was handled.
func (d *CheckerDatastore) ReadEndToEndLatency(ctx context.Context, from time.Time, to time.Time) (LatencySummary, error) {
	latencySql := fmt.Sprintf(`
		-- lab: datastore.ReadEndToEndLatency
		SELECT
			count(*),
			COALESCE(percentile_cont(0.5) WITHIN GROUP (ORDER BY latency_seconds), 0),
			COALESCE(percentile_cont(0.9) WITHIN GROUP (ORDER BY latency_seconds), 0),
			COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY latency_seconds), 0),
			COALESCE(percentile_cont(0.999) WITHIN GROUP (ORDER BY latency_seconds), 0),
			COALESCE(max(latency_seconds), 0)
		FROM (
			SELECT EXTRACT(EPOCH FROM (min(h.at) - p.scheduled_at))::double precision AS latency_seconds
			FROM %[1]s p
			JOIN %[2]s h ON h.message_id = p.message_id AND h.outcome = 'success'
			WHERE p.kind = 'committed' AND p.scheduled_at >= $1 AND p.scheduled_at < $2
			GROUP BY p.message_id, p.scheduled_at
		) AS latencies;
	`, produceRecord, handlerRecord)
	return d.readLatency(ctx, latencySql, from, to)
}

func (d *CheckerDatastore) readLatency(ctx context.Context, sql string, from time.Time, to time.Time) (LatencySummary, error) {
	var summary LatencySummary
	var p50, p90, p99, p999, max float64
	if err := d.pool.QueryRow(ctx, sql, from, to).Scan(&summary.Count, &p50, &p90, &p99, &p999, &max); err != nil {
		return LatencySummary{}, err
	}
	summary.P50 = seconds(p50)
	summary.P90 = seconds(p90)
	summary.P99 = seconds(p99)
	summary.P999 = seconds(p999)
	summary.Max = seconds(max)
	return summary, nil
}

// ReadThroughput is the committed produces per second across [from, to],
// every second present.
func (d *CheckerDatastore) ReadThroughput(ctx context.Context, from time.Time, to time.Time) ([]ThroughputSample, error) {
	throughputSql := fmt.Sprintf(`
		-- lab: datastore.ReadThroughput
		WITH committed AS (
			SELECT date_trunc('second', at) AS second, count(*) AS committed
			FROM %[1]s
			WHERE kind = 'committed'
			GROUP BY date_trunc('second', at)
		)
		SELECT s.second, COALESCE(c.committed, 0)
		FROM generate_series(date_trunc('second', $1::timestamptz), date_trunc('second', $2::timestamptz), interval '1 second') AS s(second)
		LEFT JOIN committed c ON c.second = s.second
		ORDER BY s.second;
	`, produceRecord)
	rows, err := d.pool.Query(ctx, throughputSql, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	samples := []ThroughputSample{}
	for rows.Next() {
		var sample ThroughputSample
		if err := rows.Scan(&sample.At, &sample.Committed); err != nil {
			return nil, err
		}
		samples = append(samples, sample)
	}
	return samples, rows.Err()
}

// ***************
// *** HELPERS ***
// ***************

func seconds(value float64) time.Duration {
	return time.Duration(value * float64(time.Second))
}
