package vulkan

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestConsumerGroupReadJSON(t *testing.T) {
	for _, sample := range []struct {
		name  string
		value any
	}{
		{"binding", Binding{ConsumerGroupName: "billing"}},
		{"schedule summary", ScheduleConsumerGroupSummary{ConsumerGroup: "billing"}},
		{"schedule message", ScheduleMessageStatus{ConsumerGroup: "billing"}},
	} {
		t.Run(sample.name, func(t *testing.T) {
			encoded, err := json.Marshal(sample.value)
			if err != nil {
				t.Fatal(err)
			}
			var document map[string]any
			if err := json.Unmarshal(encoded, &document); err != nil {
				t.Fatal(err)
			}
			if document["consumer_group"] != "billing" {
				t.Fatalf("consumer_group missing from %s", encoded)
			}
			if _, present := document["group"]; present {
				t.Fatalf("obsolete group key in %s", encoded)
			}
		})
	}
}

func TestConsumerErrorIdentity(t *testing.T) {
	for _, sample := range []struct {
		declaration *DiagnosticError
		code        string
	}{
		{ErrConsumerNotFound, "VK0014"},
		{ErrConsumerGroupLive, "VK0015"},
		{ErrConsumerGroupDeliveriesPending, "VK0016"},
	} {
		t.Run(sample.code, func(t *testing.T) {
			annotated := sample.declaration.With("group", "billing", "topic", "orders")
			if annotated.GetCode() != sample.code || !errors.Is(annotated, sample.declaration) {
				t.Fatalf("diagnostic identity changed: %v", annotated)
			}
		})
	}
}
