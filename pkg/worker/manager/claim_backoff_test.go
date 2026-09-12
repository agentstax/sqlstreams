package manager

import (
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/pkg/common"
)

func TestDeclinesClimbThePolicyAndClearRestartsTheStreak(t *testing.T) {
	// setup
	policy := (&common.RetryPolicy{BaseDelay: time.Second, MaxDelay: 4 * time.Second}).WithDefaults()
	backoff, err := newClaimBackoff(policy, 0)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)

	// test
	first := backoff.declined(7, start)
	second := backoff.declined(7, start)
	third := backoff.declined(7, start)
	fourth := backoff.declined(7, start)
	waitingBefore := backoff.waiting(7, start.Add(3*time.Second))
	waitingAfter := backoff.waiting(7, start.Add(4*time.Second))
	backoff.clear(7)
	restarted := backoff.declined(7, start)

	// verify
	if first != time.Second || second != 2*time.Second || third != 4*time.Second || fourth != 4*time.Second {
		t.Errorf("declined delays = %v, %v, %v, %v; want 1s, 2s, 4s, 4s", first, second, third, fourth)
	}
	if !waitingBefore {
		t.Errorf("waiting(3s after a 4s delay) = false, want true")
	}
	if waitingAfter {
		t.Errorf("waiting(4s after a 4s delay) = true, want false")
	}
	if restarted != time.Second {
		t.Errorf("declined after clear = %v, want the base delay 1s", restarted)
	}
}
