package cli

import (
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/pkg/stream"
)

func TestStreamConfigIncludesEmptyCompactionHeadTTL(t *testing.T) {
	entry, ok := findStreamConfigKey("empty_compaction_head_ttl")
	if !ok {
		t.Fatal("empty_compaction_head_ttl config key is missing")
	}
	defaults := (&stream.StreamConfig{}).WithDefaults().ToStream(0, 0, "")
	if got := entry.read(defaults); got != "1h0m0s" {
		t.Fatalf("default output = %q, want %q", got, "1h0m0s")
	}
}

func TestStreamDocumentIncludesEmptyCompactionHeadTTL(t *testing.T) {
	document := toStreamDocument(&stream.Stream{EmptyCompactionHeadTTL: 2 * time.Hour})
	if document.EmptyCompactionHeadTTL != "2h0m0s" {
		t.Fatalf("EmptyCompactionHeadTTL = %q, want %q", document.EmptyCompactionHeadTTL, "2h0m0s")
	}
}
