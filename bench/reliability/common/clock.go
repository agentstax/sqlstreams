package common

import (
	"context"
	"time"
)

// WaitUntil sleeps to instant or returns ctx.Err() first; an instant already
// past returns at once.
func WaitUntil(ctx context.Context, instant time.Time) error {
	delay := time.Until(instant)
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
