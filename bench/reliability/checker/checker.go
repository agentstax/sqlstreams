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
)

type Checker struct {
	pool        *pgxpool.Pool
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
	return &Checker{pool: pool, declared: declared, recordDir: recordDir, drainBudget: drainBudget}, nil
}

// Run loads the producers' records, drains the group, loads the handlers'
// records, runs every declared expectation, and returns the verdict. A
// returned error is a lab failure before judging began; anything that stops
// the checks short after that -- a record file that does not decode, nothing
// produced, the drain budget spent, a query failing -- is a verdict of
// unknown carrying the reason, so the record always lands.
func (c *Checker) Run(ctx context.Context) (*Verdict, error) {
	if err := c.createTables(ctx); err != nil {
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
	target, err := c.resolveTarget(ctx)
	if err != nil {
		return err
	}
	verdict.SynchronousCommit, err = c.synchronousCommit(ctx)
	if err != nil {
		return err
	}
	verdict.Records.Produce, err = c.load(ctx, produceLayout)
	if err != nil {
		return err
	}
	verdict.Records.Phase, err = c.load(ctx, phaseLayout)
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
	// handler row is on disk before the delivery it records is committed
	if err := c.drain(ctx, target); err != nil {
		return err
	}
	verdict.Records.Handler, err = c.load(ctx, handlerLayout)
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

// synchronousCommit is the server setting the verdict records: a run with
// it off proves less about durability than one with it on.
func (c *Checker) synchronousCommit(ctx context.Context) (string, error) {
	var setting string
	err := c.pool.QueryRow(ctx, "SHOW synchronous_commit;").Scan(&setting)
	return setting, err
}

// readPhases returns the run_phase rows in time order, for the results.
func (c *Checker) readPhases(ctx context.Context) ([]record.Phase, error) {
	phasesSql := fmt.Sprintf(`
		-- lab: checker.readPhases
		SELECT
			at,
			process,
			kind,
			name,
			status,
			detail
		FROM %[1]s
		ORDER BY at;
	`, runPhase)
	rows, err := c.pool.Query(ctx, phasesSql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	phases := []record.Phase{}
	for rows.Next() {
		var phase record.Phase
		if err := rows.Scan(&phase.At, &phase.Process, &phase.Kind, &phase.Name, &phase.Status, &phase.Detail); err != nil {
			return nil, err
		}
		phases = append(phases, phase)
	}
	return phases, rows.Err()
}
