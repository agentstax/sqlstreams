package controller

import (
	"testing"
	"time"
)

// invariant (one live lease): every renewal lands before the lease expires
// -- a jittered delay stays within InstanceTTL/2 * (1 ± JitterFraction),
// so the widest wait is still well inside the ttl.
func TestRenewalDelayStaysWithinTheJitteredHalfTTL(t *testing.T) {
	// setup
	ttl := 30 * time.Second
	low, high := 13500*time.Millisecond, 16500*time.Millisecond

	// test
	var outside []time.Duration
	for range 1000 {
		if delay := renewalDelay(ttl, 0.1); delay < low || delay > high {
			outside = append(outside, delay)
		}
	}

	// verify
	if len(outside) > 0 {
		t.Errorf("renewalDelay(30s, 0.1) left [%v, %v] %d times, first %v", low, high, len(outside), outside[0])
	}
	if delay := renewalDelay(ttl, 0); delay != ttl/2 {
		t.Errorf("renewalDelay(30s, 0) = %v, want exactly %v", delay, ttl/2)
	}
}
