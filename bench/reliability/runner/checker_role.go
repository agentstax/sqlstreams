package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/agentstax/vulkan/bench/reliability/checker"
)

// RunChecker judges the run from the record files the other roles left,
// writes the results under resultsDir, and prints the report. The verdict
// is returned for its exit code; a returned error is a lab failure that
// left no verdict.
func (r *Runner) RunChecker(ctx context.Context, resultsDir string, drainBudget time.Duration) (*checker.Verdict, error) {
	judge, err := checker.NewChecker(r.connection.Pool, r.declared, r.recordDir, drainBudget)
	if err != nil {
		return nil, err
	}
	verdict, err := judge.Run(ctx)
	if err != nil {
		return nil, err
	}
	written, err := checker.WriteResults(resultsDir, r.declared, verdict)
	if err != nil {
		return nil, err
	}
	fmt.Print(checker.Report(r.declared, verdict))
	fmt.Printf("\nresults %s\n", written)
	return verdict, nil
}
