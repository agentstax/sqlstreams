package controller

import (
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/pkg/stream"
)

func TestStreamConfigAdapterRoundTrip(t *testing.T) {
	cfg := (&stream.StreamConfig{
		EmptyCompactionHeadTTL: 2 * time.Hour,
	}).WithDefaults()

	row := toStreamConfigRow(7, "devices.config", cfg)
	row.Id = 41
	got, err := toStream(row)
	if err != nil {
		t.Fatalf("toStream() = %v", err)
	}
	if got.EmptyCompactionHeadTTL != 2*time.Hour {
		t.Fatalf("EmptyCompactionHeadTTL = %v, want %v", got.EmptyCompactionHeadTTL, 2*time.Hour)
	}
}
