package scenario

import (
	"fmt"
	"time"
)

// ConsumerChange sets the live consumer instance count at an offset from the
// run's start. The first change is at 0.
type ConsumerChange struct {
	At        time.Duration
	Instances int
}

func (c ConsumerChange) Validate(runDuration time.Duration) error {
	if c.At < 0 || c.At >= runDuration {
		return fmt.Errorf("At must be within the run, got %v", c.At)
	}
	if c.Instances < 0 {
		return fmt.Errorf("Instances must be >= 0, got %d", c.Instances)
	}
	return nil
}

// String is the [shape] line: "at 20m\tconsumers 0", tab-separated for the
// section's column alignment.
func (c ConsumerChange) String() string {
	return fmt.Sprintf("at %s\tconsumers %d", formatDuration(c.At), c.Instances)
}
