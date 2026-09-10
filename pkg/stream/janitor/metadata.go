package janitor

import (
	"fmt"
	"time"
)

// janitorMetadata is the config stored on the janitor worker row.
type janitorMetadata struct {
	PollRate                time.Duration `json:"poll_rate"`                  // interval between cleanup ticks
	SweepBatchSize          int           `json:"sweep_batch_size"`           // rows deleted per sweep transaction
	PartialSweepGracePeriod time.Duration `json:"partial_sweep_grace_period"` // extra retention before row cleanup; zero adds no delay
}

func (m *janitorMetadata) Validate() error {
	if m.PollRate <= 0 {
		return fmt.Errorf("poll_rate must be > 0, got %v", m.PollRate)
	}
	if m.SweepBatchSize <= 0 {
		return fmt.Errorf("sweep_batch_size must be > 0, got %d", m.SweepBatchSize)
	}
	if m.PartialSweepGracePeriod < 0 {
		return fmt.Errorf("partial_sweep_grace_period must be >= 0, got %v", m.PartialSweepGracePeriod)
	}
	return nil
}

// ***************
// *** HELPERS ***
// ***************

// defaultJanitorMetadata is the config every stream's declaration starts with.
func defaultJanitorMetadata() *janitorMetadata {
	return &janitorMetadata{
		PollRate:       5 * time.Second,
		SweepBatchSize: 1000,
	}
}
