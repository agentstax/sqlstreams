package checker

// checker judges one finished run. The loaded records (package record) say
// what the producers and handlers saw; sqlstreams's own tables say what the
// library kept. Each declared expectation is one SQL join across the two,
// returning a count and a few examples; the queries live in the datastore
// subpackage, the judgment here. Design in decision record 0687.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agentstax/sqlstreams/.bench/checker/datastore"
	"github.com/agentstax/sqlstreams/.bench/record"
	"github.com/agentstax/sqlstreams/.bench/scenario"
)

type Checker struct {
	ds          *datastore.CheckerDatastore
	declared    *scenario.Scenario
	unscaled    *scenario.Scenario
	timeScale   float64
	fingerprint *Fingerprint
	recordDir   string
	statsFile   string
	drainBudget time.Duration
}

// NewChecker judges the scaled scenario the roles ran; unscaled and
// timeScale are recorded so the run line names the workload as declared.
func NewChecker(pool *pgxpool.Pool, declared *scenario.Scenario, unscaled *scenario.Scenario, timeScale float64, fingerprint *Fingerprint, recordDir string, statsFile string, drainBudget time.Duration) (*Checker, error) {
	if pool == nil {
		return nil, errors.New("pool must not be nil")
	}
	if declared == nil {
		return nil, errors.New("declared must not be nil")
	}
	if unscaled == nil {
		return nil, errors.New("unscaled must not be nil")
	}
	if timeScale <= 0 {
		return nil, fmt.Errorf("timeScale must be > 0, got %g", timeScale)
	}
	if fingerprint == nil {
		return nil, errors.New("fingerprint must not be nil")
	}
	if recordDir == "" {
		return nil, errors.New("recordDir must not be empty")
	}
	if statsFile == "" {
		return nil, errors.New("statsFile must not be empty")
	}
	if drainBudget <= 0 {
		return nil, fmt.Errorf("drainBudget must be > 0, got %v", drainBudget)
	}

	ds, err := datastore.NewCheckerDatastore(pool, &datastore.CheckerDatastoreConfig{DisableMessageRecording: declared.DisableMessageRecording})
	if err != nil {
		return nil, err
	}
	return &Checker{ds: ds, declared: declared, unscaled: unscaled, timeScale: timeScale, fingerprint: fingerprint, recordDir: recordDir, statsFile: statsFile, drainBudget: drainBudget}, nil
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
		DisableMessageRecording: c.declared.DisableMessageRecording,
		Scenario:                c.declared.Name,
		Declaration:             c.unscaled.String(),
		TimeScale:               c.timeScale,
		StartedAt:               time.Now(),
		Fingerprint:             c.fingerprint,
		Checks:                  []CheckResult{},
		Phases:                  []record.PhaseRecord{},
	}
	if err := c.judge(ctx, verdict); err != nil {
		verdict.Status = VerdictStatusUnknown
		verdict.Reason = err.Error()
	}
	verdict.Duration = time.Since(verdict.StartedAt)
	return verdict, nil
}

func (c *Checker) judge(ctx context.Context, verdict *Verdict) error {
	targets, err := c.resolveTargets(ctx)
	if err != nil {
		return err
	}
	c.fingerprint.PostgresVersion, err = c.ds.ReadServerVersion(ctx)
	if err != nil {
		return err
	}
	c.fingerprint.Settings, err = c.ds.ReadSettings(ctx, recordedSettings)
	if err != nil {
		return err
	}
	verdict.Records.Progress, err = c.ds.LoadProgress(ctx, c.recordDir)
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
	verdict.Records.Sample, err = c.ds.LoadSample(ctx, c.recordDir)
	if err != nil {
		return err
	}
	verdict.Records.Backlog, err = c.ds.LoadBacklog(ctx, c.recordDir)
	if err != nil {
		return err
	}
	verdict.Records.Container, err = c.ds.LoadContainer(ctx, c.statsFile)
	if err != nil {
		return err
	}
	verdict.Phases, err = c.ds.ReadPhases(ctx)
	if err != nil {
		return err
	}
	verdict.Produced, err = c.ds.ReadProduceSummary(ctx)
	if err != nil {
		return err
	}
	if verdict.Produced.Committed == 0 {
		for _, phase := range c.declared.Producer {
			if phase.Unpaced || phase.Rate > 0 {
				return errors.New("nothing was produced")
			}
		}
	}

	// the produce side is measured before the drain, so a run whose
	// consumers never catch up still records what its producers saw
	verdict.Measure, err = c.measure(ctx, targets, verdict.Phases)
	if err != nil {
		return err
	}

	// the handler files are complete only once every group has drained: a
	// handler row is on disk before the delivery it records is committed
	for _, target := range targets {
		if err := c.drain(ctx, target); err != nil {
			return err
		}
	}
	if c.declared.DisableMessageRecording {
		verdict.Records.Progress, err = c.settleProgress(ctx)
		if err != nil {
			return err
		}
	}
	verdict.Records.Handler, err = c.ds.LoadHandler(ctx, c.recordDir)
	if err != nil {
		return err
	}
	verdict.Handled, err = c.ds.ReadHandlerSummary(ctx)
	if err != nil {
		return err
	}
	if err := c.measureEndToEnd(ctx, verdict.Measure, verdict.Phases); err != nil {
		return err
	}

	for _, expectation := range c.declared.Expect {
		result, err := c.check(ctx, targets, verdict.Phases, expectation)
		if err != nil {
			return fmt.Errorf("%s: %w", expectation.Check, err)
		}
		verdict.Checks = append(verdict.Checks, result)
	}
	verdict.Status = statusOf(verdict.Checks)
	return nil
}

// resolveTargets is every declared stream and group pair, in declaration
// order.
func (c *Checker) resolveTargets(ctx context.Context) ([]datastore.Target, error) {
	targets := []datastore.Target{}
	for _, declared := range c.declared.Streams {
		for _, group := range declared.Groups {
			target, err := c.ds.ResolveTarget(ctx, declared.Name, group.Name)
			if err != nil {
				return nil, err
			}
			targets = append(targets, target)
		}
	}
	return targets, nil
}

// check runs one expectation's query and judges the count against its Want.
// A per-group check is summed over every target, a per-stream check over one
// target per stream; the examples are the first targets' examples.
func (c *Checker) check(ctx context.Context, targets []datastore.Target, phases []record.PhaseRecord, expectation scenario.Expectation) (CheckResult, error) {
	if c.declared.DisableMessageRecording && expectation.Check.RequiresMessageRecording() {
		return CheckResult{Check: expectation.Check, Want: expectation.Want, Status: CheckStatusUnavailable, Reason: "per-message recording disabled"}, nil
	}
	var measured datastore.Measurement
	var err error
	switch expectation.Check {
	case scenario.CheckErrors:
		measured, err = c.ds.CountErrors(ctx)
	case scenario.CheckLost:
		measured, err = c.sumOverTargets(ctx, oneTargetPerStream(targets), c.ds.CountLost)
	case scenario.CheckUnexpected:
		measured, err = c.sumOverTargets(ctx, oneTargetPerStream(targets), c.ds.CountUnexpected)
	case scenario.CheckRecovered:
		measured, err = c.sumOverTargets(ctx, oneTargetPerStream(targets), c.ds.CountRecovered)
	case scenario.CheckUndelivered:
		measured, err = c.sumOverTargets(ctx, targets, c.ds.CountUndelivered)
	case scenario.CheckDuplicates:
		measured, err = c.sumOverTargets(ctx, targets, c.ds.CountDuplicates)
	case scenario.CheckUnbucketed:
		measured, err = c.sumOverTargets(ctx, targets, c.ds.CountUnbucketed)
	case scenario.CheckReclaims:
		measured, err = c.sumOverTargets(ctx, targets, c.ds.CountReclaims)
	case scenario.CheckDead:
		measured, err = c.sumOverTargets(ctx, targets, c.ds.CountDead)
	case scenario.CheckScheduleKept:
		measured, err = c.countScheduleSlips(ctx, phases)
	case scenario.CheckBacklogBounded:
		measured, err = c.countDivergingPhases(ctx, targets, phases)
	case scenario.CheckGeneratorHeadroom:
		measured, err = c.countHeadroomBreaches(ctx, phases)
	}
	if err != nil {
		return CheckResult{}, err
	}
	return newCheckResult(expectation, measured), nil
}

func (c *Checker) sumOverTargets(ctx context.Context, targets []datastore.Target, count func(ctx context.Context, target datastore.Target) (datastore.Measurement, error)) (datastore.Measurement, error) {
	var summed datastore.Measurement
	for _, target := range targets {
		measured, err := count(ctx, target)
		if err != nil {
			return datastore.Measurement{}, fmt.Errorf("%s/%s: %w", target.Stream, target.Group, err)
		}
		summed.Count += measured.Count
		summed.ExampleOf = measured.ExampleOf
		for _, example := range measured.Examples {
			if len(summed.Examples) < len(measured.Examples) {
				summed.Examples = append(summed.Examples, target.Stream+" "+example)
			}
		}
	}
	return summed, nil
}

// ***************
// *** HELPERS ***
// ***************

// oneTargetPerStream keeps the first target of each stream, for the checks
// that read the stream's produce records and message rows and know no group.
func oneTargetPerStream(targets []datastore.Target) []datastore.Target {
	seen := map[string]bool{}
	perStream := []datastore.Target{}
	for _, target := range targets {
		if seen[target.Stream] {
			continue
		}
		seen[target.Stream] = true
		perStream = append(perStream, target)
	}
	return perStream
}
