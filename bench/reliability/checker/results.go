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

// the results files under <dir>/<scenario>/<timestamp>/
const (
	verdictFile = "verdict.json"
	reportFile  = "report.scenario"
)

// WriteResults writes the verdict as JSON and the report columns it under
// <dir>/<scenario>/<started at>/ and returns that directory.
func WriteResults(dir string, declared *scenario.Scenario, verdict *Verdict) (string, error) {
	runDir := filepath.Join(dir, declared.Name, verdict.StartedAt.UTC().Format("20060102T150405Z"))
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return "", err
	}

	encoded, err := json.MarshalIndent(verdict, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(runDir, verdictFile), append(encoded, '\n'), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(runDir, reportFile), []byte(Report(declared, verdict)), 0o644); err != nil {
		return "", err
	}
	return runDir, nil
}

// Report is the scenario printed back with each expectation's actual columns
// it, then the produce and handler totals and the verdict line.
func Report(declared *scenario.Scenario, verdict *Verdict) string {
	columns := map[scenario.Check]string{}
	for _, check := range verdict.Checks {
		columns[check.Check] = check.ReportColumns()
	}

	var out strings.Builder
	out.WriteString(declared.Report(columns))
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

// ReportColumns is the report's columns after the declared line, tab-separated:
// the actual, then PASS or FAIL for a want of 0, then the examples of a
// non-zero count.
func (r CheckResult) ReportColumns() string {
	columns := []string{fmt.Sprintf("actual %d", r.Actual)}
	switch r.Status {
	case CheckStatusPass:
		columns = append(columns, "PASS")
	case CheckStatusFail:
		columns = append(columns, "FAIL")
	}
	if len(r.Examples) > 0 {
		columns = append(columns, r.ExampleOf+" "+strings.Join(r.Examples, ", "))
	}
	return strings.Join(columns, "\t")
}
