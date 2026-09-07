package checker

import (
	"context"

	"github.com/agentstax/vulkan/bench/reliability/scenario"
)

// witnessLimit is how many example ids or keys a check keeps beside its
// count: enough to look up, few enough to print on one line.
const witnessLimit = 5

// witness labels: what a check's example values are
const (
	witnessKey       = "key"
	witnessMessageId = "message_id"
)

// measurement is what every check query returns: how many rows matched and
// the first few, as text, so the report can name them.
type measurement struct {
	Count     int64
	Witness   string
	Witnesses []string
}

// check runs one expectation's query and judges the count against its Want.
func (c *Checker) check(ctx context.Context, target *target, expectation scenario.Expectation) (CheckResult, error) {
	var measured measurement
	var err error
	switch expectation.Check {
	case scenario.CheckLost:
		measured, err = c.lost(ctx, target)
	case scenario.CheckUnexpected:
		measured, err = c.unexpected(ctx, target)
	case scenario.CheckRecovered:
		measured, err = c.recovered(ctx, target)
	case scenario.CheckUndelivered:
		measured, err = c.undelivered(ctx, target)
	case scenario.CheckDuplicates:
		measured, err = c.duplicates(ctx)
	case scenario.CheckUnbucketed:
		measured, err = c.unbucketed(ctx, target)
	case scenario.CheckReclaims:
		measured, err = c.reclaims(ctx, target)
	case scenario.CheckDead:
		measured, err = c.dead(ctx, target)
	}
	if err != nil {
		return CheckResult{}, err
	}
	return newCheckResult(expectation, measured), nil
}

// measure runs a check query shaped `SELECT count(*), <array of witnesses>`.
func (c *Checker) measure(ctx context.Context, witness string, sql string, args ...any) (measurement, error) {
	measured := measurement{Witness: witness}
	err := c.pool.QueryRow(ctx, sql, args...).Scan(&measured.Count, &measured.Witnesses)
	return measured, err
}
