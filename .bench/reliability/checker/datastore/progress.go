package datastore

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ProgressMeasurement uses each process's actual sampled span within the window.
// Count excludes completions before its first snapshot and after its last.
type ProgressMeasurement struct {
	Count int64
	Rate  float64
}

func (d *CheckerDatastore) ReadProgressMeasurement(ctx context.Context, from time.Time, to time.Time, streamName string, producer bool) (ProgressMeasurement, error) {
	readSql := fmt.Sprintf(`
  -- lab: datastore.ReadProgressMeasurement
  SELECT COALESCE(sum(completed),0), COALESCE(sum(completed / NULLIF(seconds,0)),0),
   count(*) > 0 AND bool_and(samples >= 2 AND seconds > 0 AND completed >= 0 AND monotonic)
  FROM (
   SELECT max(committed + success) - min(committed + success) AS completed,
    EXTRACT(EPOCH FROM (max(at)-min(at)))::double precision AS seconds, count(*) AS samples, bool_and(delta IS NULL OR delta >= 0) AS monotonic
   FROM (
    SELECT *, committed + success - lag(committed + success)
     OVER (PARTITION BY process,stream,"group" ORDER BY at) AS delta
    FROM %s
    WHERE at >= $1 AND at <= $2 AND ($3 = '' OR stream = $3) AND ("group" = '') = $4
   ) snapshots
   GROUP BY process,stream,"group"
  ) counters;
 `, messageProgress)
	var measured ProgressMeasurement
	var valid bool
	err := d.pool.QueryRow(ctx, readSql, from, to, streamName, producer).Scan(&measured.Count, &measured.Rate, &valid)
	if err != nil {
		return ProgressMeasurement{}, err
	}
	if !valid {
		return ProgressMeasurement{}, errors.New("message counters require two distinct sample times and nondecreasing counts per process and stream/group")
	}
	return measured, nil
}
