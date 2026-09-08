package scenario

import (
	"errors"
	"fmt"
	"time"
)

// ProducerPhase is one open-loop stretch at a constant total rate. Phases run
// back to back and sum to the scenario's Duration.
type ProducerPhase struct {
	Name     string
	Rate     int
	Duration time.Duration
}

func (p ProducerPhase) Validate() error {
	if p.Name == "" {
		return errors.New("Name is required")
	}
	if p.Rate < 0 {
		return fmt.Errorf("Rate must be >= 0, got %d", p.Rate)
	}
	if p.Duration <= 0 {
		return fmt.Errorf("Duration must be > 0, got %v", p.Duration)
	}
	return nil
}

// String is the [shape] line after the phase's name: "steady 200/s 10m".
func (p ProducerPhase) String() string {
	return fmt.Sprintf("steady %d/s %s", p.Rate, formatDuration(p.Duration))
}

// PerTopicString is String for a scenario with several topics, each running
// the phase at this rate: "steady 200/s per topic 10m".
func (p ProducerPhase) PerTopicString() string {
	return fmt.Sprintf("steady %d/s per topic %s", p.Rate, formatDuration(p.Duration))
}
