package common

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestRetryPolicyValidatesTotalDelay(t *testing.T) {
	for _, sample := range []struct {
		name      string
		policy    RetryPolicy
		want      time.Duration
		overflows bool
	}{
		{"constant overflow", RetryPolicy{MaxRetries: 3, BaseDelay: 1 << 62, MaxDelay: 1 << 62, Exponent: 1}, 0, true},
		{"constant fits", RetryPolicy{MaxRetries: 2, BaseDelay: 1 << 62, MaxDelay: 1 << 62, Exponent: 1}, 1 << 62, false},
		{"growing fits", RetryPolicy{MaxRetries: 4, BaseDelay: 1 << 60, MaxDelay: 1 << 62, Exponent: 2}, 7 << 60, false},
		{"capped overflow", RetryPolicy{MaxRetries: 5, BaseDelay: 1 << 60, MaxDelay: 1 << 62, Exponent: 2}, 0, true},
		{"no sleeps", RetryPolicy{MaxRetries: 1, BaseDelay: 1 << 62, MaxDelay: 1 << 62, Exponent: 2}, 0, false},
		{"many tiny sleeps", RetryPolicy{MaxRetries: math.MaxInt, BaseDelay: 1, MaxDelay: 1, Exponent: 1}, time.Duration(math.MaxInt - 1), false},
		{"many capped sleeps", RetryPolicy{MaxRetries: math.MaxInt, BaseDelay: 1 << 62, MaxDelay: 1 << 62, Exponent: 2}, 0, true},
	} {
		t.Run(sample.name, func(t *testing.T) {
			err := sample.policy.Validate()
			if sample.overflows {
				if err == nil || !strings.Contains(err.Error(), "total retry delay exceeds") {
					t.Fatalf("expected total-delay validation error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := sample.policy.CalculateTotalDelay(); got != sample.want {
				t.Fatalf("validated total = %v, want %v", got, sample.want)
			}
		})
	}
}

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

func TestRetryPolicyCapsDelayBeforeConversion(t *testing.T) {
	for _, sample := range []struct {
		name    string
		base    time.Duration
		maximum time.Duration
		attempt int
		want    time.Duration
	}{
		{"first attempt", time.Second, 5 * time.Second, 0, time.Second},
		{"below cap", time.Second, 5 * time.Second, 2, 4 * time.Second},
		{"at cap", time.Second, 4 * time.Second, 2, 4 * time.Second},
		{"above cap", time.Second, 5 * time.Second, 3, 5 * time.Second},
		{"beyond duration range", time.Second, 5 * time.Minute, 40, 5 * time.Minute},
		{"infinite backoff", time.Second, 5 * time.Minute, 1024, 5 * time.Minute},
		{"maximum cap", time.Second, time.Duration(math.MaxInt64), 40, time.Duration(math.MaxInt64)},
		{"rounded cap", time.Duration(math.MaxInt64 - 1), time.Duration(math.MaxInt64 - 1), 0, time.Duration(math.MaxInt64 - 1)},
	} {
		t.Run(sample.name, func(t *testing.T) {
			policy := &RetryPolicy{MaxRetries: 1, BaseDelay: sample.base, MaxDelay: sample.maximum, Exponent: 2}
			if err := policy.Validate(); err != nil {
				t.Fatal(err)
			}
			if got := policy.CalculateDelay(sample.attempt); got != sample.want {
				t.Fatalf("delay = %v, want %v", got, sample.want)
			}
		})
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
