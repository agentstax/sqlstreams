package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/agentstax/vulkan/bench/reliability/checker"
)

// RunChecker judges the run from the ledger files the other roles left,
// writes the record under resultsDir, and prints the report. The verdict
// is returned for its exit code; a returned error is a lab failure that
// left no verdict.
func (r *Runner) RunChecker(ctx context.Context, resultsDir string, drainBudget time.Duration) (*checker.Verdict, error) {
	judge, err := checker.NewChecker(r.connection.Pool, r.declared, r.ledgerDir, drainBudget)
	if err != nil {
		return nil, err
	}
	verdict, err := judge.Run(ctx)
	if err != nil {
		return nil, err
	}
	recordDir, err := checker.WriteRecord(resultsDir, r.declared, verdict)
	if err != nil {
		return nil, err
	}
	fmt.Print(checker.Report(r.declared, verdict))
	fmt.Printf("\nrecord %s\n", recordDir)
	return verdict, nil
}
