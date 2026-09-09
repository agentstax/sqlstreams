package stream

import (
	"strings"
	"testing"
	"time"
)

func TestStreamConfigEmptyCompactionHeadTTLDefault(t *testing.T) {
	cfg := (&StreamConfig{}).WithDefaults()
	if cfg.EmptyCompactionHeadTTL != time.Hour {
		t.Fatalf("EmptyCompactionHeadTTL = %v, want %v", cfg.EmptyCompactionHeadTTL, time.Hour)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestStreamConfigRejectsNonPositiveEmptyCompactionHeadTTL(t *testing.T) {
	for _, ttl := range []time.Duration{0, -time.Nanosecond} {
		cfg := (&StreamConfig{}).WithDefaults()
		cfg.EmptyCompactionHeadTTL = ttl
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "EmptyCompactionHeadTTL must be > 0") {
			t.Errorf("Validate() with EmptyCompactionHeadTTL %v = %v, want positive-duration error", ttl, err)
		}
	}
}
