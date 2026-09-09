package janitor

import (
	"testing"
	"time"
)

func TestCleanupTimeoutConfig(t *testing.T) {
	for _, timeout := range []time.Duration{0, time.Second, -time.Second} {
		cfg := (&JanitorConfig{CleanupTimeout: timeout}).WithDefaults()
		if timeout == 0 && cfg.CleanupTimeout != 5*time.Second {
			t.Fatalf("default timeout = %v", cfg.CleanupTimeout)
		}
		if timeout != 0 && cfg.CleanupTimeout != timeout {
			t.Fatalf("explicit timeout changed to %v", cfg.CleanupTimeout)
		}
		if err := cfg.Validate(); (err != nil) != (timeout < 0) {
			t.Fatalf("timeout %v: Validate() = %v", timeout, err)
		}
	}
}
