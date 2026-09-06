package common

import (
	"testing"
	"time"
)

func TestRetryPolicyDelaySchedule(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries: 4,
		BaseDelay:  time.Second,
		MaxDelay:   5 * time.Second,
		Exponent:   2,
	}
	for attempt, want := range []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 5 * time.Second, 5 * time.Second} {
		if got := policy.CalculateDelay(attempt); got != want {
			t.Errorf("attempt %d: got %v, want %v", attempt, got, want)
		}
	}
	if got := policy.CalculateTotalDelay(); got != 7*time.Second {
		t.Fatalf("four attempts should have three sleeps totaling 7s, got %v", got)
	}
	policy.MaxRetries = 1
	if got := policy.CalculateTotalDelay(); got != 0 {
		t.Fatalf("one attempt should have no sleeps, got %v", got)
	}
	policy.MaxRetries = 5
	policy.Exponent = 1
	if got := policy.CalculateTotalDelay(); got != 4*time.Second {
		t.Fatalf("constant backoff should total 4s, got %v", got)
	}
}

func TestRetryPolicyEqualUsesStoredFields(t *testing.T) {
	var absent *RetryPolicy
	policy := NewDefaultRetryPolicy()
	if !absent.Equal(nil) || absent.Equal(policy) || policy.Equal(nil) {
		t.Fatal("nil equality must compare absence without resolving defaults")
	}
	same := *policy
	if !policy.Equal(&same) {
		t.Fatal("equal values at different addresses should compare equal")
	}
	for _, change := range []func(*RetryPolicy){
		func(policy *RetryPolicy) { policy.MaxRetries++ },
		func(policy *RetryPolicy) { policy.MaxDelays++ },
		func(policy *RetryPolicy) { policy.BaseDelay++ },
		func(policy *RetryPolicy) { policy.MaxDelay++ },
		func(policy *RetryPolicy) { policy.Exponent++ },
	} {
		different := *policy
		change(&different)
		if policy.Equal(&different) {
			t.Fatal("different stored fields should not compare equal")
		}
	}
}
