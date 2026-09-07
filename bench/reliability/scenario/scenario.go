package scenario

import (
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/agentstax/vulkan/pkg/topic"
)

// Scenario is one run's declaration: what is under test, what happens over
// time, and what must be true at the end. The .scenario file beside each
// declaration is the same content for readers; String prints this exact
// format and a test diffs the two, so the Go value is the one source.
type Scenario struct {
	Name    string
	Summary string

	Topic           string
	DeliveryLogMode topic.DeliveryLogMode
	Group           string
	HandlerFailRate float64
	MaxRetries      int
	Duration        time.Duration

	Producer  []ProducerPhase
	Consumers []ConsumerChange
	Expect    []Expectation
}

func (s *Scenario) Validate() error {
	if s.Name == "" {
		return errors.New("Name is required")
	}
	if s.Topic == "" {
		return errors.New("Topic is required")
	}
	if s.Group == "" {
		return errors.New("Group is required")
	}
	if s.HandlerFailRate < 0 || s.HandlerFailRate > 1 {
		return fmt.Errorf("HandlerFailRate must be between 0 and 1, got %g", s.HandlerFailRate)
	}
	if s.MaxRetries < 0 {
		return fmt.Errorf("MaxRetries must be >= 0, got %d", s.MaxRetries)
	}
	if s.Duration <= 0 {
		return fmt.Errorf("Duration must be > 0, got %v", s.Duration)
	}

	var phases time.Duration
	for i, phase := range s.Producer {
		if err := phase.Validate(); err != nil {
			return fmt.Errorf("Producer[%d]: %w", i, err)
		}
		phases += phase.Duration
	}
	if phases != s.Duration {
		return fmt.Errorf("Producer phases must sum to Duration %v, got %v", s.Duration, phases)
	}

	if len(s.Consumers) == 0 || s.Consumers[0].At != 0 {
		return errors.New("Consumers[0].At must be 0")
	}
	for i, change := range s.Consumers {
		if err := change.Validate(s.Duration); err != nil {
			return fmt.Errorf("Consumers[%d]: %w", i, err)
		}
	}

	if len(s.Expect) == 0 {
		return errors.New("Expect must not be empty")
	}
	declared := map[Check]bool{}
	for i, expectation := range s.Expect {
		if err := expectation.Validate(); err != nil {
			return fmt.Errorf("Expect[%d]: %w", i, err)
		}
		if declared[expectation.Check] {
			return fmt.Errorf("Expect[%d].Check already declared: %q", i, string(expectation.Check))
		}
		declared[expectation.Check] = true
	}
	return nil
}

// String prints the .scenario format: a summary comment, then the [input],
// [shape], and [expect] sections.
func (s *Scenario) String() string {
	return s.Report(nil)
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
