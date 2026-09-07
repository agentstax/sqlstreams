package checker

import (
	"context"

	"github.com/agentstax/vulkan/bench/reliability/scenario"
)

// witness labels: what a check's example values are
const (
	witnessKey       = "key"
	witnessMessageId = "message_id"
)

// check runs one expectation's query and judges the count against its Want.
func (c *Checker) check(ctx context.Context, target *target, expectation scenario.Expectation) (CheckResult, error) {
	var measured measurement
	var err error
	witness := witnessMessageId
	switch expectation.Check {
	case scenario.CheckLost:
		measured, err = c.lost(ctx, target)
		witness = witnessKey
	case scenario.CheckUnexpected:
		measured, err = c.unexpected(ctx, target)
	case scenario.CheckRecovered:
		measured, err = c.recovered(ctx, target)
		witness = witnessKey
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
	return newCheckResult(expectation, witness, measured), nil
}
