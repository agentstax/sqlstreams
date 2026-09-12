package controller

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/alert"
	"github.com/allegedlyreliable/sqlstreams/pkg/alert/evaluation"
	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/metric"
	workercontroller "github.com/allegedlyreliable/sqlstreams/pkg/worker/controller"
)

// Evaluate reads the retained stream-level unclaimed count and worker details.
// Threshold is unused; any unclaimed worker makes the condition unhealthy.
func (c *WorkerLivenessController) Evaluate(ctx context.Context, owner *common.Owner, policy *alert.JobPayload) (*alert.AlertEvaluationSnapshot, error) {
	if err := workercontroller.ValidateOwner(owner, common.OwnerStream, alert.AlertWorkerLiveness.Name); err != nil {
		return nil, err
	}
	if policy == nil {
		return nil, errors.New("policy must not be nil")
	}
	if err := policy.WithDefaults().Validate(); err != nil {
		return nil, err
	}

	key := metric.MeasurementKey(metric.MetricStreamUnclaimedWorkers.Name, map[string]string{"stream": owner.Name})
	history, err := c.metrics.GetMeasurementHistory(ctx, key, policy.Window())
	if err != nil {
		return nil, err
	}

	return c.evaluateHistory(owner, policy, history)
}

func (c *WorkerLivenessController) evaluateHistory(owner *common.Owner, policy *alert.JobPayload, history *metric.MeasurementHistory) (*alert.AlertEvaluationSnapshot, error) {
	samples := make([]*common.StoredMessage[alert.AlertEvaluationSnapshot], 0, len(history.Messages))
	for _, stored := range history.Messages {
		result, err := c.evaluateMeasurement(owner, stored.Message, stored.CreatedAt)
		if err != nil {
			return nil, err
		}
		samples = append(samples, &common.StoredMessage[alert.AlertEvaluationSnapshot]{Id: stored.Id, CreatedAt: stored.CreatedAt, Message: result})
	}
	return evaluation.EvaluateHistory(samples, history.EvaluatedAt, policy)
}

func (c *WorkerLivenessController) evaluateMeasurement(owner *common.Owner, measurement *metric.Measurement, at time.Time) (*alert.AlertEvaluationSnapshot, error) {
	// Check measurement identity and value.
	if measurement.Name != metric.MetricStreamUnclaimedWorkers.Name ||
		measurement.Kind != metric.MetricKindGauge ||
		measurement.Unit != metric.MetricUnit(metric.MetricStreamUnclaimedWorkers.Unit) {
		return alert.NewAlertEvaluationSnapshot(alert.AlertEvaluationStateInsufficientEvidence, nil, nil)
	}
	value := measurement.Value
	if math.IsNaN(value) || value < 0 || math.Trunc(value) != value {
		return alert.NewAlertEvaluationSnapshot(alert.AlertEvaluationStateInsufficientEvidence, nil, nil)
	}

	// Check that worker details account for the observed count.
	if len(measurement.Metadata) == 0 {
		return alert.NewAlertEvaluationSnapshot(alert.AlertEvaluationStateInsufficientEvidence, nil, nil)
	}
	var metadata metric.WorkerMeasurementMetadata
	if err := json.Unmarshal(measurement.Metadata, &metadata); err != nil {
		return nil, err
	}
	if metadata.Workers == nil || float64(len(metadata.Workers)) != value {
		return alert.NewAlertEvaluationSnapshot(alert.AlertEvaluationStateInsufficientEvidence, nil, nil)
	}
	for _, worker := range metadata.Workers {
		if worker == nil || worker.Name == "" || worker.Owner == nil {
			return alert.NewAlertEvaluationSnapshot(alert.AlertEvaluationStateInsufficientEvidence, nil, nil)
		}
		if worker.Owner.StreamId != owner.StreamId || worker.TargetInstances == 0 {
			return alert.NewAlertEvaluationSnapshot(alert.AlertEvaluationStateInsufficientEvidence, nil, nil)
		}
	}

	// No unclaimed workers means the stream is healthy.
	if value == 0 {
		return alert.NewAlertEvaluationSnapshot(alert.AlertEvaluationStateHealthy, nil, nil)
	}

	// Construct the active finding.
	finding, err := newWorkerLivenessAlert(owner, metadata.Workers, at)
	if err != nil {
		return nil, err
	}
	return alert.NewAlertEvaluationSnapshot(alert.AlertEvaluationStateActive, finding, nil)
}
