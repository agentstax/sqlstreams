package controller

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/pkg/alert"
	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/metric"
)

func TestWorkerHistory(t *testing.T) {
	owner, err := common.NewStreamOwner(1, 41, "orders")
	if err != nil {
		t.Fatal(err)
	}
	group, err := common.NewConsumerGroupOwner(1, 41, 7, "billing")
	if err != nil {
		t.Fatal(err)
	}
	worker, err := metric.NewUnclaimedWorkerMetadata("janitor", group, 1)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := metric.NewWorkerMeasurementMetadata([]*metric.UnclaimedWorkerMetadata{worker})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	current := time.Now()
	policy, err := alert.NewJobPayload(0, 0, 0, false)
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
		{"named group worker", string(encoded), 1, alert.AlertEvaluationStateActive, false},
		{"healthy", `{"workers":[]}`, 0, alert.AlertEvaluationStateHealthy, false},
		{"missing list", `{}`, 0, alert.AlertEvaluationStateInsufficientEvidence, false},
		{"missing metadata", "", 1, alert.AlertEvaluationStateInsufficientEvidence, false},
		{"count mismatch", string(encoded), 2, alert.AlertEvaluationStateInsufficientEvidence, false},
		{"malformed", `{`, 1, "", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			measurement, err := metric.NewBuiltInMeasurement(metric.MetricStreamUnclaimedWorkers, test.count, map[string]string{"stream": "orders"}, current)
			if err != nil {
				t.Fatal(err)
			}
			measurement.Metadata = json.RawMessage(test.metadata)
			history := &metric.MeasurementHistory{EvaluatedAt: current, Messages: []*common.StoredMessage[metric.Measurement]{
				{Id: 7106, CreatedAt: current, Message: measurement},
				{Id: 7105, CreatedAt: current.Add(-2 * time.Minute), Message: measurement},
			}}
			controller := &WorkerLivenessController{}
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
			if result.Finding != nil && (!strings.Contains(result.Finding.Detail, "billing (janitor)") || !result.Finding.At.Equal(current)) {
				t.Fatalf("finding does not retain observation details: %+v", result.Finding)
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
