package checker

import (
	"bufio"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/agentstax/vulkan/bench/reliability/scenario"
)

// runsFile is the tracked, append-only record under <dir>/<scenario>/: one
// line per run, the verdict without its phase rows and throughput series,
// which stay in the run's own directory.
const runsFile = "runs.jsonl"

// AppendRun appends the verdict's run line to <dir>/<scenario>/runs.jsonl.
func AppendRun(dir string, declared *scenario.Scenario, verdict *Verdict) error {
	line, err := runLine(verdict)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(dir, declared.Name, runsFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(append(line, '\n'))
	return err
}

// ReadRuns reads every run line of the scenario, oldest first.
func ReadRuns(dir string, scenarioName string) ([]*Verdict, error) {
	path := filepath.Join(dir, scenarioName, runsFile)
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("runs: %w -- a run through just reliability-lab writes it", err)
	}
	defer file.Close()

	runs := []*Verdict{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(nil, 1<<24)
	for scanner.Scan() {
		run := &Verdict{}
		if err := json.Unmarshal(scanner.Bytes(), run); err != nil {
			return nil, fmt.Errorf("%s line %d: %w", runsFile, len(runs)+1, err)
		}
		runs = append(runs, run)
	}
	return runs, scanner.Err()
}

// RunsReport is the runs grouped by environment identity -- library commit,
// image, and every recorded setting -- with the median of each number over
// the group's measured runs. A group of one run says so: a single run has
// no repetition behind it.
func RunsReport(scenarioName string, runs []*Verdict) string {
	groups, order := groupByIdentity(runs)

	var out strings.Builder
	fmt.Fprintf(&out, "scenario %s: %d runs, %d identities\n", scenarioName, len(runs), len(order))
	for _, key := range order {
		group := groups[key]
		out.WriteString("\n")
		out.WriteString(identityLine(group[0].Fingerprint))
		out.WriteString("\n")
		out.WriteString(groupSummaryLine(group))
		out.WriteString("\n")
		writeGroupTable(&out, group)
	}
	return out.String()
}

// ***************
// *** HELPERS ***
// ***************

// runLine is the verdict encoded without phases and the throughput series.
func runLine(verdict *Verdict) ([]byte, error) {
	encoded, err := json.Marshal(verdict)
	if err != nil {
		return nil, err
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return nil, err
	}
	delete(fields, "phases")
	if measure, ok := fields["measure"].(map[string]any); ok {
		delete(measure, "throughput")
	}
	return json.Marshal(fields)
}

// identityKey is what makes two runs comparable: the same library commit,
// the same image, the same settings read back from the server.
func identityKey(fingerprint *Fingerprint) string {
	names := make([]string, 0, len(fingerprint.Settings))
	for name := range fingerprint.Settings {
		names = append(names, name)
	}
	sort.Strings(names)
	var key strings.Builder
	fmt.Fprintf(&key, "%s %t %s", fingerprint.LibrarySha, fingerprint.LibraryDirty, fingerprint.PostgresImage)
	for _, name := range names {
		fmt.Fprintf(&key, " %s=%s", name, fingerprint.Settings[name])
	}
	return key.String()
}

func groupByIdentity(runs []*Verdict) (map[string][]*Verdict, []string) {
	groups := map[string][]*Verdict{}
	order := []string{}
	for _, run := range runs {
		key := identityKey(run.Fingerprint)
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], run)
	}
	return groups, order
}

// identityLine names the group: the commit, the image, the durability
// posture, and a short hash of every recorded setting so two groups that
// differ only in a setting read as different.
func identityLine(fingerprint *Fingerprint) string {
	settings := sha256.Sum256([]byte(identityKey(fingerprint)))
	library := fingerprint.LibrarySha
	if len(library) > 12 {
		library = library[:12]
	}
	if fingerprint.LibraryDirty {
		library += " dirty"
	}
	return fmt.Sprintf("library %s, %s, synchronous_commit %s, settings %s",
		library, fingerprint.PostgresImage, fingerprint.Settings["synchronous_commit"], hex.EncodeToString(settings[:4]))
}

func groupSummaryLine(group []*Verdict) string {
	counts := map[VerdictStatus]int{}
	for _, run := range group {
		counts[run.Status]++
	}
	line := fmt.Sprintf("  runs %d: pass %d, fail %d, unknown %d", len(group), counts[VerdictStatusPass], counts[VerdictStatusFail], counts[VerdictStatusUnknown])
	if len(group) == 1 {
		return line + " -- no rep, a single run has no spread behind it"
	}
	return line + " -- medians over the measured runs"
}

// writeGroupTable is one row per producer phase and one for the whole run,
// each number the median over the group's runs that carry a measure block.
func writeGroupTable(out *strings.Builder, group []*Verdict) {
	measured := []*Verdict{}
	for _, run := range group {
		if run.Measure != nil {
			measured = append(measured, run)
		}
	}
	if len(measured) == 0 {
		out.WriteString("  no measured run\n")
		return
	}

	table := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	fmt.Fprintln(table, "  phase\tdeclared\tachieved\tproduce p50\tp99\tend-to-end p50\tp99\tserver")
	for i, phase := range measured[0].Measure.Phases {
		fmt.Fprintf(table, "  %s\t%d/s\t%.1f/s\t%s\t%s\t%s\t%s\t\n",
			phase.Name, phase.DeclaredRate,
			medianOf(measured, func(run *Verdict) float64 { return run.Measure.Phases[i].AchievedRate }),
			formatLatency(medianDuration(measured, func(run *Verdict) time.Duration { return run.Measure.Phases[i].Produce.P50 })),
			formatLatency(medianDuration(measured, func(run *Verdict) time.Duration { return run.Measure.Phases[i].Produce.P99 })),
			formatLatency(medianDuration(measured, func(run *Verdict) time.Duration { return run.Measure.Phases[i].EndToEnd.P50 })),
			formatLatency(medianDuration(measured, func(run *Verdict) time.Duration { return run.Measure.Phases[i].EndToEnd.P99 })))
	}
	fmt.Fprintf(table, "  run\t\t\t%s\t%s\t%s\t%s\twal %.0f B/msg, %.2f transactions/msg\n",
		formatLatency(medianDuration(measured, func(run *Verdict) time.Duration { return run.Measure.Produce.P50 })),
		formatLatency(medianDuration(measured, func(run *Verdict) time.Duration { return run.Measure.Produce.P99 })),
		formatLatency(medianDuration(measured, func(run *Verdict) time.Duration { return run.Measure.EndToEnd.P50 })),
		formatLatency(medianDuration(measured, func(run *Verdict) time.Duration { return run.Measure.EndToEnd.P99 })),
		medianOf(measured, func(run *Verdict) float64 { return run.Measure.Server.WalBytesPerMessage }),
		medianOf(measured, func(run *Verdict) float64 { return run.Measure.Server.TransactionsPerMessage }))
	table.Flush()
}

func medianDuration(runs []*Verdict, read func(run *Verdict) time.Duration) time.Duration {
	return time.Duration(medianOf(runs, func(run *Verdict) float64 { return float64(read(run)) }))
}

// medianOf is the true median: the middle value, or the mean of the two
// middle values of an even count.
func medianOf(runs []*Verdict, read func(run *Verdict) float64) float64 {
	values := make([]float64, 0, len(runs))
	for _, run := range runs {
		values = append(values, read(run))
	}
	slices.SortFunc(values, cmp.Compare[float64])
	middle := len(values) / 2
	if len(values)%2 == 1 {
		return values[middle]
	}
	return (values[middle-1] + values[middle]) / 2
}
