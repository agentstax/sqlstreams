package controller

import (
	"strings"
	"testing"
	"time"
)

func TestSweepExpiredEmptyCompactionHeadsValidatesInputs(t *testing.T) {
	tests := []struct {
		name      string
		topicId   int64
		ttl       time.Duration
		batchSize int
		want      string
	}{
		{name: "invalid topic", ttl: time.Hour, batchSize: 100, want: "topicId must be > 0"},
		{name: "zero ttl", topicId: 41, batchSize: 100, want: "ttl must be > 0"},
		{name: "negative ttl", topicId: 41, ttl: -time.Second, batchSize: 100, want: "ttl must be > 0"},
		{name: "invalid batch size", topicId: 41, ttl: time.Hour, want: "batchSize must be > 0"},
	}

	controller := &JanitorController{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := controller.SweepExpiredEmptyCompactionHeads(t.Context(), test.topicId, test.ttl, test.batchSize)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("SweepExpiredEmptyCompactionHeads() = %v, want error containing %q", err, test.want)
			}
		})
	}
}
