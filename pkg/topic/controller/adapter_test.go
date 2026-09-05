package controller

import (
	"testing"
	"time"

	"github.com/agentstax/vulkan/pkg/topic"
)

func TestTopicConfigAdapterRoundTrip(t *testing.T) {
	cfg := (&topic.TopicConfig{
		EmptyCompactionHeadTTL: 2 * time.Hour,
	}).WithDefaults()

	row := toTopicConfigRow(7, "devices.config", cfg)
	row.Id = 41
	got, err := toTopic(row)
	if err != nil {
		t.Fatalf("toTopic() = %v", err)
	}
	if got.EmptyCompactionHeadTTL != 2*time.Hour {
		t.Fatalf("EmptyCompactionHeadTTL = %v, want %v", got.EmptyCompactionHeadTTL, 2*time.Hour)
	}
}
