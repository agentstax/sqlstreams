package scenario

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"text/tabwriter"
)

// Report is String with each [shape] phase line extended by the columns in
// phaseColumns, keyed by phase name, and each [expect] line by the columns
// in expectColumns, keyed by check -- "lost\t0\tactual 0\tPASS". A line with
// no entry prints as in String.
func (s *Scenario) Report(phaseColumns map[string]string, expectColumns map[Check]string) string {
	var out strings.Builder
	fmt.Fprintf(&out, "# %s\n", s.Summary)
	writeSection(&out, "[input]", s.inputLines())
	writeSection(&out, "[shape]", s.producerLines(phaseColumns))
	writeSection(&out, "", s.consumerLines())
	writeSection(&out, "[expect]", s.expectLines(expectColumns))
	return out.String()
}

// inputLines print every stream and its groups. Consecutive streams that
// differ only by a numeric suffix print as one range, `orders-1..orders-16`,
// so a family over a thousand streams reads, and ledgers, as one block.
func (s *Scenario) inputLines() []string {
	lines := []string{}
	for i := 0; i < len(s.Streams); {
		last := i
		for last+1 < len(s.Streams) && consecutiveStreams(s.Streams[last], s.Streams[last+1]) {
			last++
		}
		name := s.Streams[i].Name
		if last > i {
			name += ".." + s.Streams[last].Name
		}
		lines = append(lines, streamLines(s.Streams[i], name)...)
		i = last + 1
	}
	if s.ProducerBatchConcurrency > 0 {
		lines = append(lines, fmt.Sprintf("producer\tbatch concurrency %d per stream", s.ProducerBatchConcurrency))
	}
	if s.DisableMessageRecording {
		lines = append(lines, "recording\tper-message disabled; aggregate counters only")
	}
	if s.DisableExceptionConsumers {
		lines = append(lines, "exceptions\tdisabled")
	}
	if s.ExplicitBatching || s.ProducerConcurrency > 0 {
		lines = append(lines, fmt.Sprintf("producer\texplicit batching %t, concurrent callers %d", s.ExplicitBatching, s.ProducerConcurrency))
	}
	if s.AutomaticBatching || s.PayloadBytes > 0 || s.MaxConns > 0 {
		lines = append(lines, fmt.Sprintf("workload\tautomatic batching %t, payload bytes %d, pool max %d, batch size %d", s.AutomaticBatching, s.PayloadBytes, s.MaxConns, s.ProducerBatchSize))
	}
	return append(lines, fmt.Sprintf("duration\t%s", formatDuration(s.Duration)))
}

// producerLines say "per stream" once the rate is multiplied by more than one
// stream, so a reader never mistakes a phase's rate for the total.
func (s *Scenario) producerLines(columns map[string]string) []string {
	lines := make([]string, 0, len(s.Producer))
	for _, phase := range s.Producer {
		line := phase.Name + ":\t" + phase.String()
		if len(s.Streams) > 1 {
			line = phase.Name + ":\t" + phase.PerStreamString()
		}
		if extra, ok := columns[phase.Name]; ok {
			line += "\t" + extra
		}
		lines = append(lines, line)
	}
	return lines
}

func (s *Scenario) consumerLines() []string {
	lines := make([]string, 0, len(s.Consumers))
	for _, change := range s.Consumers {
		lines = append(lines, change.String())
	}
	return lines
}

func (s *Scenario) expectLines(columns map[Check]string) []string {
	lines := make([]string, 0, len(s.Expect))
	for _, expectation := range s.Expect {
		line := expectation.String()
		if extra, ok := columns[expectation.Check]; ok {
			line += "\t" + extra
		}
		lines = append(lines, line)
	}
	return lines
}

// ***************
// *** HELPERS ***
// ***************

// writeSection writes a blank line, the header when there is one, and the
// lines with their tab-separated columns aligned as one table.
func writeSection(out *strings.Builder, header string, lines []string) {
	out.WriteString("\n")
	if header != "" {
		fmt.Fprintln(out, header)
	}
	table := tabwriter.NewWriter(out, 0, 0, 4, ' ', 0)
	for _, line := range lines {
		fmt.Fprintln(table, line)
	}
	table.Flush()
}

// streamLines are one stream's [input] lines under name: the stream, its
// janitor and vacuum when declared, then its groups.
func streamLines(declared StreamDeclaration, name string) []string {
	line := fmt.Sprintf("stream\t%s\tDeliveryLogMode %s", name, declared.DeliveryLogMode)
	if declared.PartitionSize > 0 {
		line += fmt.Sprintf(", partition size %d", declared.PartitionSize)
	}
	if declared.RetentionTTL > 0 || declared.IdempotencyKeyTTL > 0 {
		line += fmt.Sprintf(", retention %s, key retention %s", declared.RetentionTTL, declared.IdempotencyKeyTTL)
	}
	lines := []string{line}
	if declared.Janitor != nil {
		lines = append(lines, fmt.Sprintf("janitor\t%s\tpoll %s, batch %d, grace %s, timeout %s", name, declared.Janitor.PollRate, declared.Janitor.SweepBatchSize, declared.Janitor.PartialSweepGracePeriod, declared.Janitor.CleanupTimeout))
	}
	if declared.Vacuum != nil || declared.VacuumEnabled {
		line := fmt.Sprintf("vacuum\t%s\tenabled %t", name, declared.VacuumEnabled)
		if declared.Vacuum != nil {
			line += fmt.Sprintf(", poll %s, timeout %s", declared.Vacuum.PollRate, declared.Vacuum.VacuumTimeout)
		}
		lines = append(lines, line)
	}
	for _, group := range declared.Groups {
		lines = append(lines, fmt.Sprintf("consumers\t%s\t%s", group.Name, group.String()))
	}
	return lines
}

// consecutiveStreams reports whether next is previous's declaration again
// under the next numeric suffix: `orders-3` then `orders-4`.
func consecutiveStreams(previous StreamDeclaration, next StreamDeclaration) bool {
	previousPrefix, previousNumber, ok := splitCountedName(previous.Name)
	if !ok {
		return false
	}
	nextPrefix, nextNumber, ok := splitCountedName(next.Name)
	if !ok || nextPrefix != previousPrefix || nextNumber != previousNumber+1 {
		return false
	}
	previous.Name, next.Name = "", ""
	return reflect.DeepEqual(previous, next)
}

// splitCountedName splits `orders-16` into `orders` and 16; false when the
// name has no numeric suffix.
func splitCountedName(name string) (string, int, bool) {
	dash := strings.LastIndex(name, "-")
	if dash < 0 {
		return "", 0, false
	}
	number, err := strconv.Atoi(name[dash+1:])
	if err != nil {
		return "", 0, false
	}
	return name[:dash], number, true
}
