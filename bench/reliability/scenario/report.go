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
	return []string{
		fmt.Sprintf("topic\t%s\tDeliveryLogMode %s", s.Topic, s.DeliveryLogMode),
		fmt.Sprintf("consumers\t%s\thandler fail rate %g, %d retries then dead", s.Group, s.HandlerFailRate, s.MaxRetries),
		fmt.Sprintf("duration\t%s", formatDuration(s.Duration)),
	}
}

func (s *Scenario) producerLines(columns map[string]string) []string {
	lines := make([]string, 0, len(s.Producer))
	for _, phase := range s.Producer {
		line := phase.Name + ":\t" + phase.String()
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
