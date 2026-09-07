package controller

import (
	"strings"
	"testing"

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

func TestListKeyMessagesByRankValidatesInputs(t *testing.T) {
	tests := []struct {
		name        string
		topicId     int64
		messageKey  string
		minimumRank int64
		maximumRank int64
		want        string
	}{
		{name: "zero topic", messageKey: "orders", want: "topicId must be > 0"},
		{name: "negative topic", topicId: -1, messageKey: "orders", want: "topicId must be > 0"},
		{name: "empty key", topicId: 41, want: "messageKey must not be empty"},
		{name: "reversed interval", topicId: 41, messageKey: "orders", minimumRank: 2, maximumRank: 1, want: "maximumRank must be >= minimumRank 2, got 1"},
	}
	controller := &CompactionController{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := controller.ListKeyMessagesByRank[common.RawPayload](t.Context(), test.topicId, test.messageKey, test.minimumRank, test.maximumRank)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ListKeyMessagesByRank() = %v, want %q", err, test.want)
			}
		})
	}
}
