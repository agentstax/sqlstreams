package collector

import (
	"fmt"
	"time"
)

// metricCollectorMetadata is the config stored on the metrics collector
// worker row.
type metricCollectorMetadata struct {
	PollRate time.Duration `json:"poll_rate"`
}

func (m *metricCollectorMetadata) Validate() error {
	if m.PollRate <= 0 {
		return fmt.Errorf("poll_rate must be > 0, got %v", m.PollRate)
	}
	return nil
}
