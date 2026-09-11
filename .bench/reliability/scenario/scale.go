package scenario

import "time"

// Scaled returns a copy with every duration and offset multiplied by factor,
// including declared retention and grace periods. Polls, leases, and operation
// timeouts stay fixed; shortened runs do not establish sustained capacity.
func (s *Scenario) Scaled(factor float64) *Scenario {
	scaled := *s
	scaled.Duration = scaleDuration(s.Duration, factor)
	scaled.Producer = make([]ProducerPhase, len(s.Producer))
	for i, phase := range s.Producer {
		phase.Duration = scaleDuration(phase.Duration, factor)
		scaled.Producer[i] = phase
	}
	scaled.Streams = make([]StreamDeclaration, len(s.Streams))
	for i, declared := range s.Streams {
		declared.RetentionTTL = scaleDuration(declared.RetentionTTL, factor)
		declared.IdempotencyKeyTTL = scaleDuration(declared.IdempotencyKeyTTL, factor)
		declared.Groups = append([]GroupDeclaration{}, declared.Groups...)
		if declared.Janitor != nil {
			janitor := *declared.Janitor
			janitor.PartialSweepGracePeriod = scaleDuration(janitor.PartialSweepGracePeriod, factor)
			declared.Janitor = &janitor
		}
		if declared.Vacuum != nil {
			vacuum := *declared.Vacuum
			declared.Vacuum = &vacuum
		}
		scaled.Streams[i] = declared
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
