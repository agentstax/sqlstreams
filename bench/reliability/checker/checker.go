package checker

// checker judges one finished run. The loaded records (package record) say
// what the producers and handlers saw; vulkan's own tables say what the
// library kept. Each declared expectation is one SQL join across the two,
// returning a count and a few witnesses; the queries live in the datastore
// subpackage, the judgment here. Design in decision record 0687.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agentstax/vulkan/bench/reliability/checker/datastore"
	"github.com/agentstax/vulkan/bench/reliability/record"
	"github.com/agentstax/vulkan/bench/reliability/scenario"
	"github.com/agentstax/vulkan/pkg/common"
)

type Checker struct {
	ds          *datastore.CheckerDatastore
	declared    *scenario.Scenario
	recordDir   string
	drainBudget time.Duration
}

func NewChecker(pool *pgxpool.Pool, declared *scenario.Scenario, recordDir string, drainBudget time.Duration) (*Checker, error) {
	if pool == nil {
		return nil, errors.New("pool must not be nil")
	}
	if declared == nil {
		return nil, errors.New("declared must not be nil")
	}
	if recordDir == "" {
		return nil, errors.New("recordDir must not be empty")
	}
	if drainBudget <= 0 {
		return nil, fmt.Errorf("drainBudget must be > 0, got %v", drainBudget)
	}

	ds, err := datastore.NewCheckerDatastore(pool)
	if err != nil {
		return nil, err
	}
	return &Checker{ds: ds, declared: declared, recordDir: recordDir, drainBudget: drainBudget}, nil
}

// Run loads the producers' records, drains the group, loads the handlers'
// records, runs every declared expectation, and returns the verdict. A
// returned error is a lab failure before judging began; anything that stops
// the checks short after that -- a record file that does not decode, nothing
// produced, the drain budget spent, a query failing -- is a verdict of
// unknown carrying the reason, so the record always lands.
func (c *Checker) Run(ctx context.Context) (*Verdict, error) {
	if err := c.ds.CreateTables(ctx); err != nil {
		return nil, err
	}

	verdict := &Verdict{
		Scenario:      c.declared.Name,
		StartedAt:     time.Now(),
		VulkanVersion: common.BuildVersion(),
		Checks:        []CheckResult{},
		Phases:        []record.Phase{},
	}
	if err := c.judge(ctx, verdict); err != nil {
		verdict.Status = VerdictUnknown
		verdict.Reason = err.Error()
	}
	verdict.Duration = time.Since(verdict.StartedAt)
	return verdict, nil
}

func (c *Checker) judge(ctx context.Context, verdict *Verdict) error {
	target, err := c.ds.ResolveTarget(ctx, c.declared.Topic, c.declared.Group)
	if err != nil {
		return err
	}
	verdict.SynchronousCommit, err = c.ds.SynchronousCommit(ctx)
	if err != nil {
		return err
	}
	verdict.Records.Produce, err = c.ds.LoadProduce(ctx, c.recordDir)
	if err != nil {
		return err
	}
	verdict.Records.Phase, err = c.ds.LoadPhase(ctx, c.recordDir)
	if err != nil {
		return err
	}
	verdict.Phases, err = c.ds.ReadPhases(ctx)
	if err != nil {
		return err
	}
	verdict.Produced, err = c.ds.ProduceSummary(ctx)
	if err != nil {
		return err
	}
	if verdict.Produced.Committed == 0 {
		return errors.New("nothing was produced")
	}

	// the handler files are complete only once the group has drained: a
	// handler row is on disk before the delivery it records is committed
	if err := c.drain(ctx, target); err != nil {
		return err
	}
	verdict.Records.Handler, err = c.ds.LoadHandler(ctx, c.recordDir)
	if err != nil {
		return err
	}
	verdict.Handled, err = c.ds.HandlerSummary(ctx)
	if err != nil {
		return err
	}

	for _, expectation := range c.declared.Expect {
		result, err := c.check(ctx, target, expectation)
		if err != nil {
			return fmt.Errorf("%s: %w", expectation.Check, err)
		}
		verdict.Checks = append(verdict.Checks, result)
	}
	verdict.Status = statusOf(verdict.Checks)
	return nil
}

// check runs one expectation's query and judges the count against its Want.
func (c *Checker) check(ctx context.Context, target datastore.Target, expectation scenario.Expectation) (CheckResult, error) {
	var measured datastore.Measurement
	var err error
	switch expectation.Check {
	case scenario.CheckLost:
		measured, err = c.ds.Lost(ctx, target)
	case scenario.CheckUnexpected:
		measured, err = c.ds.Unexpected(ctx, target)
	case scenario.CheckRecovered:
		measured, err = c.ds.Recovered(ctx, target)
	case scenario.CheckUndelivered:
		measured, err = c.ds.Undelivered(ctx, target)
	case scenario.CheckDuplicates:
		measured, err = c.ds.Duplicates(ctx)
	case scenario.CheckUnbucketed:
		measured, err = c.ds.Unbucketed(ctx, target)
	case scenario.CheckReclaims:
		measured, err = c.ds.Reclaims(ctx, target)
	case scenario.CheckDead:
		measured, err = c.ds.Dead(ctx, target)
	}
	if err != nil {
		return CheckResult{}, err
	}
	return newCheckResult(expectation, measured), nil
}
