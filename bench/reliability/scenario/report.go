package scenario

import (
	"fmt"
	"strings"
	"text/tabwriter"
)

// Report is String with each [expect] line extended by the columns in
// beside, keyed by check -- "lost\t0\tactual 0\tPASS". A check with no
// entry prints as in String.
func (s *Scenario) Report(beside map[Check]string) string {
	var out strings.Builder
	fmt.Fprintf(&out, "# %s\n", s.Summary)
	writeSection(&out, "[input]", s.inputLines())
	writeSection(&out, "[shape]", s.producerLines())
	writeSection(&out, "", s.consumerLines())
	writeSection(&out, "[expect]", s.expectLines(beside))
	return out.String()
}

func (s *Scenario) inputLines() []string {
	return []string{
		fmt.Sprintf("topic\t%s\tDeliveryLogMode %s", s.Topic, s.DeliveryLogMode),
		fmt.Sprintf("consumers\t%s\thandler fail rate %g, %d retries then dead", s.Group, s.HandlerFailRate, s.MaxRetries),
		fmt.Sprintf("duration\t%s", formatDuration(s.Duration)),
	}
}

func (s *Scenario) producerLines() []string {
	lines := make([]string, 0, len(s.Producer))
	for _, phase := range s.Producer {
		lines = append(lines, phase.Name+":\t"+phase.String())
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

func (s *Scenario) expectLines(beside map[Check]string) []string {
	lines := make([]string, 0, len(s.Expect))
	for _, expectation := range s.Expect {
		line := expectation.String()
		if columns, ok := beside[expectation.Check]; ok {
			line += "\t" + columns
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
