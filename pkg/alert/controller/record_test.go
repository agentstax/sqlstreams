package controller

import (
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/pkg/alert"
	"github.com/agentstax/sqlstreams/pkg/common"
)

func TestRecordLeavesInconclusiveEvidenceUnchanged(t *testing.T) {
	owner, err := common.NewStreamOwner(1, 41, "orders")
	if err != nil {
		t.Fatal(err)
	}
	finding, err := alert.NewAlert("partition_count", owner, alert.AlertStatusActive, alert.AlertSeverityWarn, "partition count exceeds threshold", time.Now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []alert.AlertEvaluationState{alert.AlertEvaluationStatePending, alert.AlertEvaluationStateInsufficientEvidence} {
		t.Run(string(state), func(t *testing.T) {
			var found *alert.Alert
			if state == alert.AlertEvaluationStatePending {
				found = finding
			}
			result, err := alert.NewAlertEvaluationSnapshot(state, found, nil)
			if err != nil {
				t.Fatal(err)
			}
			// No datastore or producer: either access would panic.
			controller := &AlertController{}
			outcome, err := controller.Record(t.Context(), "partition_count", owner, result)
			if err != nil || outcome != alert.RecordOutcomeNothing {
				t.Fatalf("Record() = %q, %v; want nothing", outcome, err)
			}
		})
	}
}

func TestRecordRejectsMalformedResultsBeforeReading(t *testing.T) {
	owner, err := common.NewStreamOwner(1, 41, "orders")
	if err != nil {
		t.Fatal(err)
	}
	controller := &AlertController{}
	for _, result := range []*alert.AlertEvaluationSnapshot{
		nil,
		{},
		{State: alert.AlertEvaluationStateActive},
		{State: alert.AlertEvaluationStateHealthy, Finding: &alert.Alert{Status: alert.AlertStatusActive}},
	} {
		if _, err := controller.Record(t.Context(), "partition_count", owner, result); err == nil {
			t.Fatalf("Record(%+v) must reject malformed result", result)
		}
	}
}
