package checker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/agentstax/vulkan/bench/reliability/scenario"
)

// record file names under <dir>/<scenario>/<timestamp>/
const (
	verdictFile = "verdict.json"
	reportFile  = "report.scenario"
)

// WriteResults writes the verdict as JSON and the report beside it under
// <dir>/<scenario>/<started at>/ and returns that directory.
func WriteResults(dir string, declared *scenario.Scenario, verdict *Verdict) (string, error) {
	recordDir := filepath.Join(dir, declared.Name, verdict.StartedAt.UTC().Format("20060102T150405Z"))
	if err := os.MkdirAll(recordDir, 0o755); err != nil {
		return "", err
	}

	encoded, err := json.MarshalIndent(verdict, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(recordDir, verdictFile), append(encoded, '\n'), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(recordDir, reportFile), []byte(Report(declared, verdict)), 0o644); err != nil {
		return "", err
	}
	return recordDir, nil
}

// Report is the scenario printed back with each expectation's actual beside
// it, then the produce and handler totals and the verdict line.
func Report(declared *scenario.Scenario, verdict *Verdict) string {
	beside := map[scenario.Check]string{}
	for _, check := range verdict.Checks {
		beside[check.Check] = check.Beside()
	}

	var out strings.Builder
	out.WriteString(declared.Report(beside))
	out.WriteString("\n")
	table := tabwriter.NewWriter(&out, 0, 0, 4, ' ', 0)
	fmt.Fprintf(table, "records\t%d produce, %d handler, %d phase rows\n", verdict.Records.Produce, verdict.Records.Handler, verdict.Records.Phase)
	fmt.Fprintf(table, "produced\tattempted %d, committed %d, rejected %d, unknown %d\n",
		verdict.Produced.Attempted, verdict.Produced.Committed, verdict.Produced.Rejected, verdict.Produced.Unknown)
	fmt.Fprintf(table, "handled\tsuccess %d, error %d\n", verdict.Handled.Success, verdict.Handled.Error)
	fmt.Fprintf(table, "verdict\t%s\t%s\n", verdict.Status, verdict.Reason)
	table.Flush()
	return out.String()
}
