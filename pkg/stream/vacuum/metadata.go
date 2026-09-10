package vacuum

import (
	"fmt"
	"time"
)

// vacuumMetadata is the config stored on the vacuum worker row.
type vacuumMetadata struct {
	PollRate      time.Duration `json:"poll_rate"`      // delay after each request
	VacuumTimeout time.Duration `json:"vacuum_timeout"` // maximum duration of a request, including retries
}

func (m *vacuumMetadata) Validate() error {
	if m.PollRate <= 0 {
		return fmt.Errorf("poll_rate must be > 0, got %v", m.PollRate)
	}
	if m.VacuumTimeout <= 0 {
		return fmt.Errorf("vacuum_timeout must be > 0, got %v", m.VacuumTimeout)
	}
	return nil
}
