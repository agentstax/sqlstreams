package controller

import (
	"strings"
	"testing"
	"time"

	"github.com/agentstax/vulkan/pkg/common"
)

func TestListKeyMessagesRejectsNonPositiveLimit(t *testing.T) {
	controller := &CompactionController{}
	for _, limit := range []int{0, -1} {
		_, err := controller.ListKeyMessages[common.RawPayload](t.Context(), 41, "orders", limit)
		if err == nil {
			t.Errorf("limit %d did not return an error", limit)
		}
	}
}

func TestListKeyMessagesByCreatedAtValidatesInputs(t *testing.T) {
	start := time.Date(2026, time.September, 7, 10, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Minute)
	tests := []struct {
		name       string
		topicId    int64
		messageKey string
		start      time.Time
		end        time.Time
		want       string
	}{
		{name: "zero topic", messageKey: "orders", want: "topicId must be > 0"},
		{name: "negative topic", topicId: -1, messageKey: "orders", want: "topicId must be > 0"},
		{name: "empty key", topicId: 41, want: "messageKey must not be empty"},
		{name: "missing start", topicId: 41, messageKey: "orders", end: end, want: "start must not be zero"},
		{name: "missing end", topicId: 41, messageKey: "orders", start: start, want: "end must not be zero"},
		{name: "reversed interval", topicId: 41, messageKey: "orders", start: end, end: start, want: "end must be >= start"},
	}
	controller := &CompactionController{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := controller.ListKeyMessagesByCreatedAt[common.RawPayload](t.Context(), test.topicId, test.messageKey, test.start, test.end)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ListKeyMessagesByCreatedAt() = %v, want %q", err, test.want)
			}
		})
	}
}
