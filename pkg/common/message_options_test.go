package common

import (
	"testing"
	"time"
)

func TestMessageOptionsResolutionPreservesInputs(t *testing.T) {
	scheduledAt := time.Date(2026, 9, 6, 9, 0, 0, 0, time.UTC)
	requested := &MessageOptions{
		Concurrency: ConcurrencyExclusive,
		Timeout:     20 * time.Second,
		Retry:       &RetryPolicy{MaxRetries: 5},
		ScheduledAt: scheduledAt,
	}
	defaults := &MessageOptions{
		Timeout:     30 * time.Second,
		Retry:       &RetryPolicy{MaxRetries: 3, BaseDelay: time.Second},
		ScheduledAt: scheduledAt.Add(time.Hour),
	}
	maximum := &MessageOptions{Timeout: 10 * time.Second, Retry: &RetryPolicy{MaxRetries: 4}}
	filled := requested.Fill(defaults)
	clamped := filled.Clamp(nil, maximum)
	resolved := clamped.ResolveConcurrency(ConcurrencyParallel)
	if resolved.Timeout != 10*time.Second || resolved.Concurrency != ConcurrencyParallel ||
		resolved.Retry.MaxRetries != 4 || resolved.Retry.BaseDelay != time.Second ||
		!resolved.ScheduledAt.Equal(scheduledAt) {
		t.Fatalf("unexpected resolved options: %+v, retry: %+v", resolved, resolved.Retry)
	}
	resolved.Retry.MaxRetries = 8
	if clamped.Retry.MaxRetries != 8 {
		t.Fatal("ResolveConcurrency should share the retry pointer")
	}
	if filled.Retry.MaxRetries != 5 || requested.Retry.MaxRetries != 5 ||
		defaults.Retry.MaxRetries != 3 || maximum.Retry.MaxRetries != 4 {
		t.Fatal("Fill or Clamp retained an input retry pointer")
	}
	if requested.Timeout != 20*time.Second || clamped.Concurrency != ConcurrencyExclusive {
		t.Fatal("resolution changed an input struct")
	}
	if !(&MessageOptions{}).Fill(defaults).ScheduledAt.IsZero() {
		t.Fatal("Fill must not inherit ScheduledAt from defaults")
	}
}

func TestMessageOptionsNilAndConcurrency(t *testing.T) {
	var absent *MessageOptions
	if absent.Fill(nil) != nil || absent.Clamp(nil, nil) != nil {
		t.Fatal("nil options should remain nil without fill defaults")
	}
	if absent.ResolveConcurrency("").Concurrency != ConcurrencyParallel {
		t.Fatal("nil options should resolve to parallel")
	}
	if absent.ResolveConcurrency(ConcurrencyExclusive).Concurrency != ConcurrencyExclusive {
		t.Fatal("override should apply to nil options")
	}
	requested := &MessageOptions{Concurrency: ConcurrencyExclusive, Timeout: time.Second}
	if requested.ResolveConcurrency("").Concurrency != ConcurrencyExclusive {
		t.Fatal("empty override should preserve the request")
	}
	if requested.Clamp(nil, nil).Timeout != time.Second {
		t.Fatal("unset bounds should leave timeout unchanged")
	}
	if requested.Clamp(&MessageOptions{Timeout: 2 * time.Second}, nil).Timeout != 2*time.Second {
		t.Fatal("positive lower bound should raise timeout")
	}
}

func TestMessageOptionsEqualUsesInstantsAndStoredValues(t *testing.T) {
	var absent *MessageOptions
	if !absent.Equal(nil) || absent.Equal(&MessageOptions{}) || (&MessageOptions{}).Equal(nil) {
		t.Fatal("nil equality should distinguish absence from empty options")
	}
	instant := time.Date(2026, 9, 6, 9, 0, 0, 0, time.UTC)
	left := &MessageOptions{ScheduledAt: instant, Retry: &RetryPolicy{MaxRetries: 3}}
	right := &MessageOptions{
		ScheduledAt: instant.In(time.FixedZone("example", -4*60*60)),
		Retry:       &RetryPolicy{MaxRetries: 3},
	}
	if !left.Equal(right) {
		t.Fatal("equal instants and retry values should compare equal")
	}
	right.ScheduledAt = instant.Add(time.Second)
	if left.Equal(right) {
		t.Fatal("different scheduled instants should compare unequal")
	}
	right.ScheduledAt = instant
	right.Retry.MaxDelays = 1
	if left.Equal(right) {
		t.Fatal("different retry fields should compare unequal")
	}
}
