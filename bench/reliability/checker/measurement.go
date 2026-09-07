package checker

import (
	"context"
)

// witnessLimit is how many example ids or keys a check keeps beside its
// count: enough to look up, few enough to print on one line.
const witnessLimit = 5

// measurement is what every check query returns: how many rows matched and
// the first few, as text, so the report can name them.
type measurement struct {
	Count     int64
	Witnesses []string
}

// measure runs a check query shaped `SELECT count(*), <array of witnesses>`.
func (c *Checker) measure(ctx context.Context, sql string, args ...any) (measurement, error) {
	var measured measurement
	err := c.pool.QueryRow(ctx, sql, args...).Scan(&measured.Count, &measured.Witnesses)
	return measured, err
}
