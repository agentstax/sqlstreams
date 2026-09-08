package scenario

import "time"

// Scaled returns a copy with every duration and offset multiplied by factor,
// so the hour-long declaration runs in a minute on a laptop. Only the
// timeline scales: lease and timeout durations stay what the library gives,
// which is why a short run can pass without ever crossing a lease expiry.
func (s *Scenario) Scaled(factor float64) *Scenario {
	scaled := *s
	scaled.Duration = scaleDuration(s.Duration, factor)
	scaled.Producer = make([]ProducerPhase, len(s.Producer))
	for i, phase := range s.Producer {
		phase.Duration = scaleDuration(phase.Duration, factor)
		scaled.Producer[i] = phase
	}
	scaled.Consumers = make([]ConsumerChange, len(s.Consumers))
	for i, change := range s.Consumers {
		change.At = scaleDuration(change.At, factor)
		scaled.Consumers[i] = change
	}
	return &scaled
}

func scaleDuration(duration time.Duration, factor float64) time.Duration {
	return time.Duration(float64(duration) * factor)
}
