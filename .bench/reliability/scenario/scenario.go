package scenario

import (
	"errors"
	"fmt"
	"time"
)

// Scenario is one run's declaration: what is under test, what happens over
// time, and what must be true at the end. The .scenario file columns each
// declaration is the same content for readers; String prints this exact
// format and a test diffs the two, so the Go value is the one source.
type Scenario struct {
	Name     string
	Summary  string
	Duration time.Duration

	// every stream runs Producer's phases, each at the phase's rate; every
	// consumer process runs Consumers' instance count on every group
	Streams   []StreamDeclaration
	Producer  []ProducerPhase
	Consumers []ConsumerChange
	Expect    []Expectation

	// ProducerBatchConcurrency is each stream's producer batch workers, one
	// connection each; 0 leaves the library's default
	ProducerBatchConcurrency int
	ProducerBatchSize        int
	AutomaticBatching        bool
	PayloadBytes             int
	MaxConns                 int
}

func (s *Scenario) Validate() error {
	if s.Name == "" {
		return errors.New("Name is required")
	}
	if s.Duration <= 0 {
		return fmt.Errorf("Duration must be > 0, got %v", s.Duration)
	}
	if s.ProducerBatchConcurrency < 0 {
		return fmt.Errorf("ProducerBatchConcurrency must be >= 0, got %d", s.ProducerBatchConcurrency)
	}
	if s.ProducerBatchSize < 0 {
		return fmt.Errorf("ProducerBatchSize must be >= 0, got %d", s.ProducerBatchSize)
	}
	if s.PayloadBytes != 0 && s.PayloadBytes < 128 {
		return fmt.Errorf("PayloadBytes must be 0 or >= 128, got %d", s.PayloadBytes)
	}
	if s.MaxConns < 0 {
		return fmt.Errorf("MaxConns must be >= 0, got %d", s.MaxConns)
	}
	if len(s.Streams) == 0 {
		return errors.New("Streams must not be empty")
	}
	streams := map[string]bool{}
	for i, declared := range s.Streams {
		if err := declared.Validate(); err != nil {
			return fmt.Errorf("Streams[%d]: %w", i, err)
		}
		if streams[declared.Name] {
			return fmt.Errorf("Streams[%d].Name already declared: %q", i, declared.Name)
		}
		streams[declared.Name] = true
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
	return s.Report(nil, nil)
}
