package checker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/agentstax/vulkan/.bench/reliability/checker/datastore"
	"github.com/agentstax/vulkan/.bench/reliability/scenario"
)

// the results files under <dir>/<scenario>/<timestamp>/
const (
	verdictFile = "verdict.json"
	reportFile  = "report.scenario"
)

// WriteResults writes the verdict as JSON, the report columns it, and
// copies of the observer's and the host's raw sample files under
// <dir>/<scenario>/<started at>/ and returns that directory.
func WriteResults(dir string, recordDir string, statsFile string, declared *scenario.Scenario, verdict *Verdict) (string, error) {
	runDir := filepath.Join(dir, declared.Name, verdict.StartedAt.UTC().Format("20060102T150405Z"))
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return "", err
	}

	samples, err := filepath.Glob(filepath.Join(recordDir, "*.sample.jsonl"))
	if err != nil {
		return "", err
	}
	backlogs, err := filepath.Glob(filepath.Join(recordDir, "*.backlog.jsonl"))
	if err != nil {
		return "", err
	}
	for _, path := range append(append(samples, backlogs...), statsFile) {
		if err := copyFile(path, filepath.Join(runDir, filepath.Base(path))); err != nil {
			return "", err
		}
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

// Report is the scenario printed back with each phase's measured columns and
// each expectation's actual columns, then the produce and handler totals,
// the whole-run latency, and the verdict line.
func Report(declared *scenario.Scenario, verdict *Verdict) string {
	phaseColumns := map[string]string{}
	expectColumns := map[scenario.Check]string{}
	if verdict.Measure != nil {
		for _, phase := range verdict.Measure.Phases {
			phaseColumns[phase.Name] = phase.ReportColumns()
		}
	}
	for _, check := range verdict.Checks {
		expectColumns[check.Check] = check.ReportColumns()
	}

	var out strings.Builder
	out.WriteString(declared.Report(phaseColumns, expectColumns))
	out.WriteString("\n")
	table := tabwriter.NewWriter(&out, 0, 0, 4, ' ', 0)
	fmt.Fprintf(table, "records\t%d produce, %d handler, %d phase, %d sample, %d backlog, %d container rows\n",
		verdict.Records.Produce, verdict.Records.Handler, verdict.Records.Phase, verdict.Records.Sample, verdict.Records.Backlog, verdict.Records.Container)
	fmt.Fprintf(table, "produced\tattempted %d, committed %d, rejected %d, unknown %d\n",
		verdict.Produced.Attempted, verdict.Produced.Committed, verdict.Produced.Rejected, verdict.Produced.Unknown)
	fmt.Fprintf(table, "handled\tsuccess %d, error %d\n", verdict.Handled.Success, verdict.Handled.Error)
	if verdict.Measure != nil {
		fmt.Fprintf(table, "produce\t%s\n", latencyColumns(verdict.Measure.Produce))
		fmt.Fprintf(table, "end-to-end\t%s\n", latencyColumns(verdict.Measure.EndToEnd))
		fmt.Fprintf(table, "server\t%s\n", serverColumns(verdict.Measure.Server))
	}
	fmt.Fprintf(table, "environment\t%s\n", environmentLine(verdict.Fingerprint))
	fmt.Fprintf(table, "verdict\t%s\t%s\n", verdict.Status, verdict.Reason)
	table.Flush()
	return out.String()
}

// ReportColumns is the report's columns after the phase's [shape] line,
// tab-separated: the achieved total rate, produce and end-to-end p50 and
// p99, each service's median CPU, then held or the guard that gave.
func (p PhaseSummary) ReportColumns() string {
	guards := "held"
	if !p.Held {
		guards = fmt.Sprintf("gave: backlog %+.1f/s, slips %ds, headroom %d", p.BacklogSlope, p.ScheduleSlips, p.HeadroomBreaches)
	}
	return fmt.Sprintf("achieved %.1f/s of %d/s\tproduce p50 %s p99 %s\tend-to-end p50 %s p99 %s\tcpu postgres %.0f%% producer %.0f%% consumer %.0f%%\t%s",
		p.AchievedRate, p.DeclaredRate,
		formatLatency(p.Produce.P50), formatLatency(p.Produce.P99),
		formatLatency(p.EndToEnd.P50), formatLatency(p.EndToEnd.P99),
		p.PostgresCpu, p.ProducerCpu, p.ConsumerCpu, guards)
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

// ***************
// *** HELPERS ***
// ***************

// latencyColumns is a whole-run latency line: every percentile and the max.
func latencyColumns(summary datastore.LatencySummary) string {
	return fmt.Sprintf("p50 %s, p90 %s, p99 %s, p99.9 %s, max %s, over %d",
		formatLatency(summary.P50), formatLatency(summary.P90), formatLatency(summary.P99),
		formatLatency(summary.P999), formatLatency(summary.Max), summary.Count)
}

// serverColumns is the server's cost line: per-message WAL and
// transactions, then the checkpoints and deadlocks in the window.
func serverColumns(server ServerSummary) string {
	return fmt.Sprintf("wal %.0f B/msg, %.2f records/msg, %.3f fpi/msg, %.2f transactions/msg, checkpoints %d, deadlocks %d",
		server.WalBytesPerMessage, server.WalRecordsPerMessage, server.WalFpiPerMessage, server.TransactionsPerMessage,
		server.Deltas.Checkpoints, server.Deltas.Deadlocks)
}

// copyFile copies src to dst whole; the raw sample files are small.
func copyFile(src string, dst string) error {
	content, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, content, 0o644)
}

// formatLatency prints milliseconds below a second, seconds above.
func formatLatency(latency time.Duration) string {
	if latency >= time.Second {
		return fmt.Sprintf("%.2fs", latency.Seconds())
	}
	return fmt.Sprintf("%.1fms", float64(latency)/float64(time.Millisecond))
}

// environmentLine is the fingerprint's one-line form: the server version and
// durability posture the run had, and the library commit that ran.
func environmentLine(fingerprint *Fingerprint) string {
	library := fingerprint.LibrarySha
	if len(library) > 12 {
		library = library[:12]
	}
	if fingerprint.LibraryDirty {
		library += " dirty"
	}
	return fmt.Sprintf("postgres %s, synchronous_commit %s, library %s",
		fingerprint.PostgresVersion, fingerprint.Settings["synchronous_commit"], library)
}
