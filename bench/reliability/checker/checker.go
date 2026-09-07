package checker

// checker judges one finished run. The loaded records (package record) say
// what the producers and handlers saw; vulkan's own tables say what the
// library kept. Each declared expectation is one SQL join across the two,
// returning a count and a few witnesses. Design in decision record 0687.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agentstax/vulkan/bench/reliability/record"
	"github.com/agentstax/vulkan/bench/reliability/scenario"
	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/topic"
)

type Checker struct {
	pool        *pgxpool.Pool
	declared    *scenario.Scenario
	tables      *tables
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

	tables, err := newTables(pool)
	if err != nil {
		return nil, err
	}
	return &Checker{pool: pool, declared: declared, tables: tables, recordDir: recordDir, drainBudget: drainBudget}, nil
}

// Run loads the producers' records, drains the group, loads the handlers'
// records, runs every declared expectation, and returns the verdict. A
// returned error is a lab failure before judging began; anything that stops
// the checks short after that -- nothing produced, the drain budget spent, a
// query failing -- is a verdict of unknown carrying the reason, so the
// record always lands.
func (c *Checker) Run(ctx context.Context) (*Verdict, error) {
	if err := c.tables.create(ctx); err != nil {
		return nil, err
	}
	verdict := &Verdict{
		Scenario:      c.declared.Name,
		StartedAt:     time.Now(),
		VulkanVersion: common.BuildVersion(),
		Checks:        []CheckResult{},
		Phases:        []record.Phase{},
	}
	var err error
	verdict.Records.Produce, err = c.tables.load(ctx, c.recordDir, record.FileProduce)
	if err != nil {
		return nil, err
	}
	verdict.Records.Phase, err = c.tables.load(ctx, c.recordDir, record.FilePhase)
	if err != nil {
		return nil, err
	}

	if err := c.judge(ctx, verdict); err != nil {
		verdict.Status = StatusUnknown
		verdict.Reason = err.Error()
	}
	verdict.Duration = time.Since(verdict.StartedAt)
	return verdict, nil
}

func (c *Checker) judge(ctx context.Context, verdict *Verdict) error {
	if c.declared.DeliveryLogMode != topic.DeliveryLogModeAll {
		return fmt.Errorf("the delivery checks need DeliveryLogMode %q, got %q", topic.DeliveryLogModeAll, c.declared.DeliveryLogMode)
	}
	target, err := c.resolveTarget(ctx)
	if err != nil {
		return err
	}
	verdict.SynchronousCommit, err = c.synchronousCommit(ctx)
	if err != nil {
		return err
	}
	verdict.Phases, err = c.readPhases(ctx)
	if err != nil {
		return err
	}
	verdict.Produced, err = c.produceSummary(ctx)
	if err != nil {
		return err
	}
	if verdict.Produced.Committed == 0 {
		return errors.New("nothing was produced")
	}

	// the handler files are complete only once the group has drained: a
	// handler fact is on disk before the delivery it records is committed
	if err := c.drain(ctx, target); err != nil {
		return err
	}
	verdict.Records.Handler, err = c.tables.load(ctx, c.recordDir, record.FileHandler)
	if err != nil {
		return err
	}
	verdict.Handled, err = c.handlerSummary(ctx)
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
