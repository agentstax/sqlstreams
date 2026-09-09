package sqlstreamstest

import (
	"os"
	"testing"
	"time"
)

const waitForInterval = 10 * time.Millisecond

// A CI runner is slower and shared, so its deadline is the longer one.
const (
	waitForDeadline   = 3 * time.Second
	waitForDeadlineCI = 10 * time.Second
)

// WaitFor calls condition until it returns nil. Past the deadline it fails
// the test with condition's last error; it never hangs.
func WaitFor(t testing.TB, condition func() error) {
	t.Helper()
	deadline := waitForDeadline
	if os.Getenv("CI") != "" {
		deadline = waitForDeadlineCI
	}

	timer := time.NewTimer(deadline)
	defer timer.Stop()
	ticker := time.NewTicker(waitForInterval)
	defer ticker.Stop()
	err := condition()
	for err != nil {
		select {
		case <-timer.C:
			t.Fatalf("condition not met within %v: %v", deadline, err)
		case <-ticker.C:
			err = condition()
		}
	}
}
