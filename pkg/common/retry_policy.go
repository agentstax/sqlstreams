package common

import (
	"fmt"
	"math"
	"time"
)

const MIN_DELAY = 0

// RetryPolicy is an exponential backoff curve. ClientConfig.Retry applies it
// to the client's own Postgres calls; MessageOptions.Retry applies it to a
// message's redelivery. Zero fields take the defaults.
type RetryPolicy struct {
	// MaxRetries - failed attempts allowed, the first included: a Postgres
	// call gives up, a delivery dead-letters, once it is reached.
	// Default: 6 (3 as a consumer's Message.Retry).
	MaxRetries int `json:"max_retries,omitempty"`

	// MaxDelays - handler-requested later runs (consume.Delay) before the
	// delivery dead-letters. Redelivery only; meaningless for Postgres calls.
	// Default: 0 (no cap).
	MaxDelays int `json:"max_delays,omitempty"`

	// BaseDelay - the first backoff delay.
	// Default: 1s.
	BaseDelay time.Duration `json:"base_delay,omitempty"`

	// MaxDelay - the ceiling every later delay is capped at.
	// Default: 5m.
	MaxDelay time.Duration `json:"max_delay,omitempty"`

	// Exponent - the per-attempt multiplier: delay = BaseDelay * Exponent^attempt.
	// Default: 2.
	Exponent int `json:"exponent,omitempty"`
}

func NewDefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxRetries: 6,
		BaseDelay:  time.Second,
		MaxDelay:   5 * time.Minute,
		Exponent:   2,
	}
}

// CalculateDelay returns BaseDelay * Exponent^attempt, capped at MaxDelay.
// Use a defaulted, valid policy and a zero-based, nonnegative attempt; MaxRetries is not enforced.
func (p *RetryPolicy) CalculateDelay(attempt int) time.Duration {
	delay := float64(p.BaseDelay) * math.Pow(float64(p.Exponent), float64(attempt))
	if delay >= float64(p.MaxDelay) {
		return p.MaxDelay
	}
	return time.Duration(delay)
}

// CalculateTotalDelay sums sleeps before the final datastore attempt, excluding
// operation time and early exits. Call WithDefaults and Validate before calculating.
func (p *RetryPolicy) CalculateTotalDelay() time.Duration {
	var total time.Duration
	for attempt := range p.MaxRetries - 1 {
		delay := p.CalculateDelay(attempt)
		// Constant backoff and the capped tail need only one multiplication.
		if p.Exponent == 1 || delay == p.MaxDelay {
			return total + delay*time.Duration(p.MaxRetries-1-attempt)
		}
		total += delay
	}
	return total
}

// Equal compares stored fields, including MaxDelays, without resolving defaults.
// Two nil policies are equal; nil and an explicit default policy are not.
func (p *RetryPolicy) Equal(other *RetryPolicy) bool {
	if p == nil || other == nil {
		return p == other
	}
	return *p == *other
}

func (p *RetryPolicy) WithDefaults() *RetryPolicy {
	if p == nil {
		return NewDefaultRetryPolicy()
	}

	// set defaults for any non-set values
	defaults := NewDefaultRetryPolicy()
	if p.MaxRetries == 0 {
		p.MaxRetries = defaults.MaxRetries
	}
	if p.BaseDelay == 0 {
		p.BaseDelay = defaults.BaseDelay
	}
	if p.MaxDelay == 0 {
		p.MaxDelay = defaults.MaxDelay
	}
	if p.Exponent == 0 {
		p.Exponent = defaults.Exponent
	}
	return p
}

func (p *RetryPolicy) Validate() error {
	if p == nil {
		return nil // nil is valid -- it resolves to the default policy at use
	}

	// MaxRetries < 1 makes Wrap's loop run ZERO times -- it would return nil
	// without ever calling the wrapped func, a silent fake success
	if p.MaxRetries < 1 {
		return fmt.Errorf("MaxRetries must be >= 1, got %d", p.MaxRetries)
	}
	if p.MaxDelays < 0 {
		return fmt.Errorf("MaxDelays must be >= 0, got %d", p.MaxDelays)
	}

	// non-positive BaseDelay/MaxDelay clamp every backoff to 0 -- transient
	// errors would retry in a hot loop
	if p.BaseDelay <= 0 {
		return fmt.Errorf("BaseDelay must be > 0, got %v", p.BaseDelay)
	}
	if p.MaxDelay <= 0 {
		return fmt.Errorf("MaxDelay must be > 0, got %v", p.MaxDelay)
	}
	if p.MaxDelay < p.BaseDelay {
		return fmt.Errorf("MaxDelay (%v) must be >= BaseDelay (%v)", p.MaxDelay, p.BaseDelay)
	}

	// Exponent < 1 flips CalculateDelay's sign on alternating attempts
	if p.Exponent < 1 {
		return fmt.Errorf("Exponent must be >= 1, got %d", p.Exponent)
	}

	remaining := time.Duration(math.MaxInt64)
	for attempt := range p.MaxRetries - 1 {
		delay := p.CalculateDelay(attempt)
		count := 1
		if p.Exponent == 1 || delay == p.MaxDelay {
			count = p.MaxRetries - 1 - attempt
		}
		if delay > 0 && time.Duration(count) > remaining/delay {
			return fmt.Errorf("total retry delay exceeds maximum supported duration (%s)", time.Duration(math.MaxInt64))
		}
		remaining -= delay * time.Duration(count)
		if count > 1 {
			break
		}
	}
	return nil
}
