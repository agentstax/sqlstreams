package alert

import (
	"fmt"
	"math"
	"time"
)

type JobPayload struct {
	Threshold       int64         `json:"threshold"`   // 0 = Evaluate derives the alert's live default
	MaximumAge      time.Duration `json:"maximum_age"` // 0 = collector progress derives its live default
	PendingDuration time.Duration `json:"pending_duration"`
	MaximumGap      time.Duration `json:"maximum_gap"`
	DisablePending  bool          `json:"disable_pending"`
}

func (JobPayload) SchemaVersion() int { return 1 }

func NewJobPayload(threshold int64, pendingDuration time.Duration, maximumGap time.Duration, disablePending bool) (*JobPayload, error) {
	data := (&JobPayload{Threshold: threshold, PendingDuration: pendingDuration, MaximumGap: maximumGap, DisablePending: disablePending}).WithDefaults()
	if err := data.Validate(); err != nil {
		return nil, err
	}
	return data, nil
}

func (d *JobPayload) WithDefaults() *JobPayload {
	if d.PendingDuration == 0 {
		d.PendingDuration = 2 * time.Minute
	}
	if d.MaximumGap == 0 {
		d.MaximumGap = 2 * time.Minute
	}
	return d
}

func (d *JobPayload) Validate() error {
	if d.Threshold < 0 {
		return fmt.Errorf("threshold must be >= 0, got %d", d.Threshold)
	}
	if d.MaximumAge < 0 {
		return fmt.Errorf("maximum_age must be >= 0, got %v", d.MaximumAge)
	}
	if d.PendingDuration <= 0 {
		return fmt.Errorf("PendingDuration must be > 0, got %v", d.PendingDuration)
	}
	if d.MaximumGap <= 0 {
		return fmt.Errorf("MaximumGap must be > 0, got %v", d.MaximumGap)
	}
	if !d.DisablePending && d.MaximumGap > (math.MaxInt64-d.PendingDuration)/2 {
		return fmt.Errorf("MaximumGap must be <= %v for PendingDuration %v, got %v", (time.Duration(math.MaxInt64)-d.PendingDuration)/2, d.PendingDuration, d.MaximumGap)
	}
	return nil
}

// Window includes a predecessor across the duration boundary. Validate first.
func (d *JobPayload) Window() time.Duration {
	if d.DisablePending {
		return d.MaximumGap
	}
	return d.PendingDuration + 2*d.MaximumGap
}
