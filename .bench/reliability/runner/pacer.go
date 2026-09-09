package runner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/agentstax/sqlstreams/.bench/reliability/common"
)

// Pacer runs an open loop: call i is due at start + i/rate whether or not
// call i-1 has returned, so a stalled database shows up as late calls, never
// as a quieter producer. InFlight bounds the goroutines a stall can pile up.
type Pacer struct {
	rate     int
	duration time.Duration
	inFlight int
}

func NewPacer(rate int, duration time.Duration, inFlight int) (*Pacer, error) {
	if rate < 0 {
		return nil, fmt.Errorf("rate must be >= 0, got %d", rate)
	}
	if duration <= 0 {
		return nil, fmt.Errorf("duration must be > 0, got %v", duration)
	}
	if inFlight <= 0 {
		return nil, fmt.Errorf("inFlight must be > 0, got %d", inFlight)
	}
	return &Pacer{rate: rate, duration: duration, inFlight: inFlight}, nil
}

// Run fires call at every due instant until the phase ends, then waits for
// the calls still running. A rate of 0 is a pause: Run sleeps the duration.
// Returns ctx.Err() when cancelled early.
func (p *Pacer) Run(ctx context.Context, call func(ctx context.Context, scheduled time.Time)) error {
	start := time.Now()
	end := start.Add(p.duration)
	if p.rate == 0 {
		return common.WaitUntil(ctx, end)
	}

	interval := time.Second / time.Duration(p.rate)
	permits := make(chan struct{}, p.inFlight)
	var running sync.WaitGroup
	defer running.Wait()
	for i := 0; ; i++ {
		scheduled := start.Add(time.Duration(i) * interval)
		if !scheduled.Before(end) {
			return nil
		}
		if err := common.WaitUntil(ctx, scheduled); err != nil {
			return err
		}

		select {
		case permits <- struct{}{}:
		case <-ctx.Done():
			return ctx.Err()
		}
		running.Go(func() {
			defer func() { <-permits }()
			call(ctx, scheduled)
		})
	}
}
