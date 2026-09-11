package scenario

import (
	"errors"
	"fmt"
	"time"
)

// ProducerPhase declares a paced, idle, or unpaced interval.
// Phases run back to back and sum to the scenario duration.
type ProducerPhase struct {
	Name   string
	Rate   int
	Warmup bool
	// Unpaced runs continuously; Rate 0 without it remains an idle phase.
	Unpaced  bool
	Duration time.Duration
}

func (p ProducerPhase) Validate() error {
	if p.Name == "" {
		return errors.New("Name is required")
	}
	if p.Unpaced && p.Rate != 0 {
		return errors.New("Unpaced requires Rate 0")
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
	if p.Unpaced {
		if p.Warmup {
			return fmt.Sprintf("warmup unpaced %s", formatDuration(p.Duration))
		}
		return fmt.Sprintf("unpaced %s", formatDuration(p.Duration))
	}
	if p.Warmup {
		return fmt.Sprintf("warmup steady %d/s %s", p.Rate, formatDuration(p.Duration))
	}
	return fmt.Sprintf("steady %d/s %s", p.Rate, formatDuration(p.Duration))
}

// PerStreamString is String for a scenario with several streams, each running
// the phase at this rate: "steady 200/s per stream 10m".
func (p ProducerPhase) PerStreamString() string {
	if p.Unpaced {
		if p.Warmup {
			return fmt.Sprintf("warmup unpaced per stream %s", formatDuration(p.Duration))
		}
		return fmt.Sprintf("unpaced per stream %s", formatDuration(p.Duration))
	}
	if p.Warmup {
		return fmt.Sprintf("warmup steady %d/s per stream %s", p.Rate, formatDuration(p.Duration))
	}
	return fmt.Sprintf("steady %d/s per stream %s", p.Rate, formatDuration(p.Duration))
}
