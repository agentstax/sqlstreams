package scenario

import (
	"errors"
	"fmt"
	"time"

	"github.com/agentstax/vulkan/pkg/topic"
)

// Scenario is one run's declaration: what is under test, what happens over
// time, and what must be true at the end. The .scenario file columns each
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

	// unbucketed reads delivery_log success rows, which only mode all writes
	if s.DeliveryLogMode != topic.DeliveryLogModeAll {
		return fmt.Errorf("DeliveryLogMode must be %q, got %q", topic.DeliveryLogModeAll, s.DeliveryLogMode)
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
	// the checker drains on the group's cursor, which only a running
	// consumer advances
	if s.Consumers[len(s.Consumers)-1].Instances == 0 {
		return errors.New("Consumers must end with at least one instance running")
	}

	declared := map[Check]Want{}
	for i, expectation := range s.Expect {
		if err := expectation.Validate(); err != nil {
			return fmt.Errorf("Expect[%d]: %w", i, err)
		}
		if _, ok := declared[expectation.Check]; ok {
			return fmt.Errorf("Expect[%d].Check already declared: %q", i, string(expectation.Check))
		}
		declared[expectation.Check] = expectation.Want
	}
	for _, invariant := range Invariants {
		want, ok := declared[invariant.Check]
		if !ok {
			return fmt.Errorf("Expect must declare %s %s", invariant.Check, invariant.Want)
		}
		if want != invariant.Want {
			return fmt.Errorf("Expect must declare %s %s, got %q", invariant.Check, invariant.Want, want)
		}
	}
	return nil
}

// String prints the .scenario format: a summary comment, then the [input],
// [shape], and [expect] sections.
func (s *Scenario) String() string {
	return s.Report(nil)
}
