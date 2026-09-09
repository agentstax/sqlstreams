package common

import (
	"math"
	"testing"
	"time"
)

// closed set: the delay for each attempt of an exponential curve, capped at
// MaxDelay, and the total sleep across the curve.
func TestRetryPolicyDelayScheduleCapsAtMaxDelay(t *testing.T) {
	policy := &RetryPolicy{MaxRetries: 4, BaseDelay: time.Second, MaxDelay: 5 * time.Second, Exponent: 2}
	tests := []struct {
		name    string
		attempt int
		want    time.Duration
	}{
		{name: "first", attempt: 0, want: time.Second},
		{name: "doubled", attempt: 1, want: 2 * time.Second},
		{name: "doubled again", attempt: 2, want: 4 * time.Second},
		{name: "capped", attempt: 3, want: 5 * time.Second},
		{name: "past the curve", attempt: 4, want: 5 * time.Second},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := policy.CalculateDelay(test.attempt); got != test.want {
				t.Fatalf("CalculateDelay(%d) = %v, want %v", test.attempt, got, test.want)
			}
		})
	}

	// four attempts sleep three times: 1s + 2s + 4s
	if got := policy.CalculateTotalDelay(); got != 7*time.Second {
		t.Fatalf("CalculateTotalDelay() = %v, want 7s", got)
	}
}

// invariant: the cap is applied before the exponent overflows a Duration, so
// a late attempt never wraps negative or past MaxDelay.
func TestRetryPolicyCapsDelayBeforeOverflow(t *testing.T) {
	tests := []struct {
		name    string
		base    time.Duration
		maximum time.Duration
		attempt int
		want    time.Duration
	}{
		{name: "beyond duration range", base: time.Second, maximum: 5 * time.Minute, attempt: 40, want: 5 * time.Minute},
		{name: "exponent overflows int", base: time.Second, maximum: 5 * time.Minute, attempt: 1024, want: 5 * time.Minute},
		{name: "cap is the maximum duration", base: time.Second, maximum: time.Duration(math.MaxInt64), attempt: 40, want: time.Duration(math.MaxInt64)},
		{name: "base is one below the maximum", base: time.Duration(math.MaxInt64 - 1), maximum: time.Duration(math.MaxInt64 - 1), attempt: 0, want: time.Duration(math.MaxInt64 - 1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := &RetryPolicy{MaxRetries: 1, BaseDelay: test.base, MaxDelay: test.maximum, Exponent: 2}
			if err := policy.Validate(); err != nil {
				t.Fatal(err)
			}
			if got := policy.CalculateDelay(test.attempt); got != test.want {
				t.Fatalf("CalculateDelay(%d) = %v, want %v", test.attempt, got, test.want)
			}
		})
	}
}

// closed set: Validate refuses a curve whose total sleep overflows a
// Duration, and accepts one that fits exactly.
func TestRetryPolicyValidateBoundsTotalDelay(t *testing.T) {
	tests := []struct {
		name      string
		policy    RetryPolicy
		wantTotal time.Duration
		wantErr   bool
	}{
		{name: "constant overflow", policy: RetryPolicy{MaxRetries: 3, BaseDelay: 1 << 62, MaxDelay: 1 << 62, Exponent: 1}, wantErr: true},
		{name: "constant fits", policy: RetryPolicy{MaxRetries: 2, BaseDelay: 1 << 62, MaxDelay: 1 << 62, Exponent: 1}, wantTotal: 1 << 62},
		{name: "growing fits", policy: RetryPolicy{MaxRetries: 4, BaseDelay: 1 << 60, MaxDelay: 1 << 62, Exponent: 2}, wantTotal: 7 << 60},
		{name: "capped overflow", policy: RetryPolicy{MaxRetries: 5, BaseDelay: 1 << 60, MaxDelay: 1 << 62, Exponent: 2}, wantErr: true},
		{name: "single attempt never sleeps", policy: RetryPolicy{MaxRetries: 1, BaseDelay: 1 << 62, MaxDelay: 1 << 62, Exponent: 2}, wantTotal: 0},
		{name: "many tiny sleeps", policy: RetryPolicy{MaxRetries: math.MaxInt, BaseDelay: 1, MaxDelay: 1, Exponent: 1}, wantTotal: time.Duration(math.MaxInt - 1)},
		{name: "many capped sleeps", policy: RetryPolicy{MaxRetries: math.MaxInt, BaseDelay: 1 << 62, MaxDelay: 1 << 62, Exponent: 2}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.policy.Validate()
			if (err != nil) != test.wantErr {
				t.Fatalf("Validate(%s) = %v, want error %v", test.name, err, test.wantErr)
			}
			if test.wantErr {
				return
			}
			if got := test.policy.CalculateTotalDelay(); got != test.wantTotal {
				t.Fatalf("CalculateTotalDelay(%s) = %v, want %v", test.name, got, test.wantTotal)
			}
		})
	}
}
