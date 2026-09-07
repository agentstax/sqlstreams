package collectorprogress

import (
	"fmt"
	"time"
)

// collectorProgressMetadata is the config stored on the alert's worker row.
type collectorProgressMetadata struct {
	RepeatInterval time.Duration `json:"repeat_interval"`
}

func (m *collectorProgressMetadata) Validate() error {
	if m.RepeatInterval <= 0 {
		return fmt.Errorf("repeat_interval must be > 0, got %v", m.RepeatInterval)
	}
	return nil
}
