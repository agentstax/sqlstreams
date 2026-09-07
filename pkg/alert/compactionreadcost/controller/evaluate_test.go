package controller

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/agentstax/vulkan/pkg/alert"
	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/metrics"
)

func TestCompactionHistory(t *testing.T) {
	owner, err := common.NewTopicOwner(1, 41, "orders")
	if err != nil {
		t.Fatal(err)
	}
	current := time.Now()
	policy, err := alert.NewJobPayload(100, 0, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		metadata  string
		count     float64
		state     alert.AlertEvaluationState
		wantError bool
	}{
		{"compacted", `{"compaction_status":"compacted"}`, 105, alert.AlertEvaluationStateActive, false},
		{"uncompacted", `{"compaction_status":"uncompacted"}`, 105, alert.AlertEvaluationStateHealthy, false},
		{"below threshold", `{"compaction_status":"compacted"}`, 95, alert.AlertEvaluationStateHealthy, false},
		{"missing metadata", "", 105, alert.AlertEvaluationStateInsufficientEvidence, false},
		{"empty metadata", `{}`, 105, alert.AlertEvaluationStateInsufficientEvidence, false},
		{"malformed metadata", `{`, 105, "", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			measurement, err := metrics.NewBuiltInMeasurement(metrics.MetricTopicPartitions, test.count, map[string]string{"topic": "orders"}, current)
			if err != nil {
				t.Fatal(err)
			}
			measurement.Metadata = json.RawMessage(test.metadata)
			history := &metrics.MeasurementHistory{EvaluatedAt: current, Messages: []*common.StoredMessage[metrics.Measurement]{
				{Id: 7106, CreatedAt: current, Message: measurement},
				{Id: 7105, CreatedAt: current.Add(-2 * time.Minute), Message: measurement},
			}}
			controller := &CompactionReadCostController{}
			result, err := controller.evaluateHistory(owner, policy, history)
			if test.wantError {
				if err == nil {
					t.Fatal("want decode error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.State != test.state {
				t.Fatalf("got %s, want %s", result.State, test.state)
			}
			history.Messages = history.Messages[:1]
			result, err = controller.evaluateHistory(owner, policy, history)
			if err != nil {
				t.Fatal(err)
			}
			if test.state == alert.AlertEvaluationStateActive && result.State != alert.AlertEvaluationStatePending {
				t.Fatal("first unhealthy sample must be pending")
			}
		})
	}
}
