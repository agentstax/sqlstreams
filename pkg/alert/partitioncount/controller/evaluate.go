package controller

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/agentstax/vulkan/pkg/alert"
	"github.com/agentstax/vulkan/pkg/alert/evaluation"
	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/metrics"
)

// warnDivisor halves the lock ceiling so the alert leaves headroom to act
// before Destroy starts failing.
const warnDivisor = 2

// Evaluate derives pending from retained partition counts, using storage time.
// Threshold 0 uses half the live lock ceiling; missing evidence cannot resolve.
func (c *PartitionCountController) Evaluate(ctx context.Context, owner *common.Owner, policy *alert.JobPayload) (*alert.AlertEvaluationResult, error) {
	if owner == nil {
		return nil, errors.New("owner must not be nil")
	}
	if policy == nil {
		return nil, errors.New("policy must not be nil")
	}
	if err := policy.WithDefaults().Validate(); err != nil {
		return nil, err
	}

	ceiling, err := c.datastore.PartitionLockCeiling(ctx)
	if err != nil {
		return nil, err
	}

	key := metrics.MeasurementKey(metrics.MetricTopicPartitions.Name, map[string]string{"topic": owner.Name})
	history, err := c.metrics.GetMeasurementHistory(ctx, key, policy.Window())
	if err != nil {
		return nil, err
	}
	return c.evaluateHistory(owner, policy, ceiling, history)
}

func (c *PartitionCountController) evaluateHistory(owner *common.Owner, policy *alert.JobPayload, ceiling int64, history *metrics.MeasurementHistory) (*alert.AlertEvaluationResult, error) {
	threshold := policy.Threshold
	if threshold == 0 {
		threshold = ceiling / warnDivisor
	}
	samples := make([]*common.StoredMessage[alert.AlertEvaluationResult], 0, len(history.Messages))
	for _, stored := range history.Messages {
		result, err := c.evaluateMeasurement(owner, threshold, ceiling, stored.Message, stored.CreatedAt)
		if err != nil {
			return nil, err
		}
		samples = append(samples, &common.StoredMessage[alert.AlertEvaluationResult]{Id: stored.Id, CreatedAt: stored.CreatedAt, Message: result})
	}
	return evaluation.EvaluateHistory(samples, history.EvaluatedAt, policy)
}

func (c *PartitionCountController) evaluateMeasurement(owner *common.Owner, threshold int64, ceiling int64, measurement *metrics.Measurement, at time.Time) (*alert.AlertEvaluationResult, error) {
	// Check measurement identity and value.
	if measurement.Name != metrics.MetricTopicPartitions.Name ||
		measurement.Kind != metrics.MetricKindGauge ||
		measurement.Unit != metrics.MetricUnit(metrics.MetricTopicPartitions.Unit) {
		return alert.NewAlertEvaluationResult(alert.AlertEvaluationStateInsufficientEvidence, nil)
	}
	value := measurement.Value
	if math.IsNaN(value) || value < 0 || value >= math.MaxInt64 || math.Trunc(value) != value {
		return alert.NewAlertEvaluationResult(alert.AlertEvaluationStateInsufficientEvidence, nil)
	}

	// Counts below the threshold are healthy.
	count := int64(value)
	if count < threshold {
		return alert.NewAlertEvaluationResult(alert.AlertEvaluationStateHealthy, nil)
	}

	// Construct the active finding.
	finding, err := newPartitionCountAlert(owner, count, ceiling, threshold, at)
	if err != nil {
		return nil, err
	}
	return alert.NewAlertEvaluationResult(alert.AlertEvaluationStateActive, finding)
}
