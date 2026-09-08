package scenario

import (
	"fmt"
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

func (s *Scenario) inputLines() []string {
	lines := []string{}
	for _, declared := range s.Topics {
		lines = append(lines, fmt.Sprintf("topic\t%s\tDeliveryLogMode %s", declared.Name, declared.DeliveryLogMode))
		for _, group := range declared.Groups {
			lines = append(lines, fmt.Sprintf("consumers\t%s\t%s", group.Name, group.String()))
		}
	}
	if s.ProducerBatchConcurrency > 0 {
		lines = append(lines, fmt.Sprintf("producer\tbatch concurrency %d per topic", s.ProducerBatchConcurrency))
	}
	if s.AutomaticBatching || s.PayloadBytes > 0 || s.MaxConns > 0 {
		lines = append(lines, fmt.Sprintf("workload\tautomatic batching %t, payload bytes %d, pool max %d, batch size %d", s.AutomaticBatching, s.PayloadBytes, s.MaxConns, s.ProducerBatchSize))
	}
	return append(lines, fmt.Sprintf("duration\t%s", formatDuration(s.Duration)))
}

// producerLines say "per topic" once the rate is multiplied by more than one
// topic, so a reader never mistakes a phase's rate for the total.
func (s *Scenario) producerLines(columns map[string]string) []string {
	lines := make([]string, 0, len(s.Producer))
	for _, phase := range s.Producer {
		line := phase.Name + ":\t" + phase.String()
		if len(s.Topics) > 1 {
			line = phase.Name + ":\t" + phase.PerTopicString()
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
