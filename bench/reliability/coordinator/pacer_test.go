package coordinator

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestPacerFiresOnceInterval(t *testing.T) {
	pacer, err := NewPacer(100, 200*time.Millisecond, 8)
	if err != nil {
		t.Fatal(err)
	}

	var calls atomic.Int64
	err = pacer.Run(context.Background(), func(ctx context.Context, scheduled time.Time) {
		calls.Add(1)
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 20 {
		t.Fatalf("100/s for 200ms fired %d calls, want 20", got)
	}
}

func TestPacerDoesNotWaitForSlowCalls(t *testing.T) {
	pacer, err := NewPacer(100, 100*time.Millisecond, 64)
	if err != nil {
		t.Fatal(err)
	}

	started := time.Now()
	var calls atomic.Int64
	err = pacer.Run(context.Background(), func(ctx context.Context, scheduled time.Time) {
		calls.Add(1)
		time.Sleep(150 * time.Millisecond)
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 10 {
		t.Fatalf("open loop fired %d calls, want 10 regardless of call latency", got)
	}
	if elapsed := time.Since(started); elapsed > 400*time.Millisecond {
		t.Fatalf("Run took %v; the calls ran serially instead of in flight together", elapsed)
	}
}

func TestPacerRateZeroIsAPause(t *testing.T) {
	pacer, err := NewPacer(0, 50*time.Millisecond, 1)
	if err != nil {
		t.Fatal(err)
	}

	started := time.Now()
	err = pacer.Run(context.Background(), func(ctx context.Context, scheduled time.Time) {
		t.Error("a pause fired a call")
	})
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed < 50*time.Millisecond {
		t.Fatalf("pause returned after %v, want >= 50ms", elapsed)
	}
}

func TestPacerReturnsCtxErrWhenCancelled(t *testing.T) {
	pacer, err := NewPacer(10, time.Minute, 1)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err = pacer.Run(ctx, func(ctx context.Context, scheduled time.Time) {})
	if err != context.DeadlineExceeded {
		t.Fatalf("Run returned %v, want context.DeadlineExceeded", err)
	}
}
