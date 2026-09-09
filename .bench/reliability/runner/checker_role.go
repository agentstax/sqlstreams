package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/agentstax/sqlstreams/.bench/reliability/checker"
)

// RunChecker judges the run from the record files the other roles left and
// the fingerprint the recipe wrote, writes the results under resultsDir,
// appends the run line, and prints the report. The verdict is returned for its exit code; a returned
// error is a lab failure that left no verdict.
func (r *Runner) RunChecker(ctx context.Context, resultsDir string, fingerprintFile string, statsFile string, drainBudget time.Duration) (*checker.Verdict, error) {
	fingerprint, err := checker.ReadFingerprint(fingerprintFile)
	if err != nil {
		return nil, err
	}
	judge, err := checker.NewChecker(r.connection.Pool, r.declared, r.unscaled, r.timeScale, fingerprint, r.recordDir, statsFile, drainBudget)
	if err != nil {
		return nil, err
	}
	verdict, err := judge.Run(ctx)
	if err != nil {
		return nil, err
	}
	written, err := checker.WriteResults(resultsDir, r.recordDir, statsFile, r.declared, verdict)
	if err != nil {
		return nil, err
	}
	if err := checker.AppendRun(resultsDir, r.declared, verdict); err != nil {
		return nil, err
	}
	fmt.Print(checker.Report(r.declared, verdict))
	fmt.Printf("\nresults %s\n", written)
	return verdict, nil
}
