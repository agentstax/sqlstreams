package datastore

import (
	"context"
	"fmt"
	"time"
)

// StatementCost is one statement shape's cost over a window: the calls,
// execution time, and rows pg_stat_statements added between the window's
// first and last sample.
type StatementCost struct {
	Query  string  `json:"query"`
	Calls  int64   `json:"calls"`
	ExecMs float64 `json:"exec_ms"`
	Rows   int64   `json:"rows"`
}

// ReadStatementDeltas is every statement shape's cost over [from, to),
// costliest first. The counters are cumulative, so the window's difference
// is its maximum less its minimum; a shape whose calls did not move is
// left out.
func (d *CheckerDatastore) ReadStatementDeltas(ctx context.Context, from time.Time, to time.Time) ([]StatementCost, error) {
	deltasSql := fmt.Sprintf(`
		-- lab: datastore.ReadStatementDeltas
		SELECT
			query,
			max(calls) - min(calls),
			max(exec_ms) - min(exec_ms),
			max(rows) - min(rows)
		FROM %[1]s
		WHERE at >= $1 AND at < $2
		GROUP BY query
		HAVING max(calls) > min(calls)
		ORDER BY max(exec_ms) - min(exec_ms) DESC, query;
	`, observerStatement)
	rows, err := d.pool.Query(ctx, deltasSql, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	costs := []StatementCost{}
	for rows.Next() {
		var cost StatementCost
		if err := rows.Scan(&cost.Query, &cost.Calls, &cost.ExecMs, &cost.Rows); err != nil {
			return nil, err
		}
		costs = append(costs, cost)
	}
	return costs, rows.Err()
}

// ReadWorkerRows counts the deployment's worker_config rows: the fleet the
// run's upkeep was paying for.
func (d *CheckerDatastore) ReadWorkerRows(ctx context.Context) (int64, error) {
	countSql := fmt.Sprintf(`
		-- lab: datastore.ReadWorkerRows
		SELECT count(*) FROM %[1]s.worker_config;
	`, sqlstreamsSchema)
	var count int64
	err := d.pool.QueryRow(ctx, countSql).Scan(&count)
	return count, err
}
