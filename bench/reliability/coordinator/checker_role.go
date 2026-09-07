package coordinator

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
func (c *Coordinator) RunChecker(ctx context.Context, resultsDir string, drainBudget time.Duration) (*checker.Verdict, error) {
	judge, err := checker.NewChecker(c.connection.Pool, c.declared, c.ledgerDir, drainBudget)
	if err != nil {
		return nil, err
	}
	verdict, err := judge.Run(ctx)
	if err != nil {
		return nil, err
	}
	recordDir, err := checker.WriteRecord(resultsDir, c.declared, verdict)
	if err != nil {
		return nil, err
	}
	fmt.Print(checker.Report(c.declared, verdict))
	fmt.Printf("\nrecord %s\n", recordDir)
	return verdict, nil
}
