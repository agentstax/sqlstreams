package controller

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/agentstax/vulkan/pkg/alert"
	"github.com/agentstax/vulkan/pkg/alert/evaluation"
	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/metrics"
	workercontroller "github.com/agentstax/vulkan/pkg/worker/controller"
)

// warnPartitions is where one never-superseded key's replay, at ~10µs per
// partition, crosses ~100ms.
const warnPartitions = 10_000

// Evaluate reads partition count and compaction applicability from one retained
// measurement. Threshold 0 uses warnPartitions; missing metadata is insufficient.
func (c *CompactionReadCostController) Evaluate(ctx context.Context, owner *common.Owner, policy *alert.JobPayload) (*alert.AlertEvaluationSnapshot, error) {
	if err := workercontroller.ValidateOwner(owner, common.OwnerTopic, alert.AlertCompactionReadCost.Name); err != nil {
		return nil, err
	}
	if policy == nil {
		return nil, errors.New("policy must not be nil")
	}
	if err := policy.WithDefaults().Validate(); err != nil {
		return nil, err
	}

	key := metrics.MeasurementKey(metrics.MetricTopicPartitions.Name, map[string]string{"topic": owner.Name})
	history, err := c.metrics.GetMeasurementHistory(ctx, key, policy.Window())
	if err != nil {
		return nil, err
	}
	return c.evaluateHistory(owner, policy, history)
}

func (c *CompactionReadCostController) evaluateHistory(owner *common.Owner, policy *alert.JobPayload, history *metrics.MeasurementHistory) (*alert.AlertEvaluationSnapshot, error) {
	threshold := policy.Threshold
	if threshold == 0 {
		threshold = warnPartitions
	}

	samples := make([]*common.StoredMessage[alert.AlertEvaluationSnapshot], 0, len(history.Messages))
	for _, stored := range history.Messages {
		result, err := c.evaluateMeasurement(owner, threshold, stored.Message, stored.CreatedAt)
		if err != nil {
			return nil, err
		}
		samples = append(samples, &common.StoredMessage[alert.AlertEvaluationSnapshot]{Id: stored.Id, CreatedAt: stored.CreatedAt, Message: result})
	}
	return evaluation.EvaluateHistory(samples, history.EvaluatedAt, policy)
}

func (c *CompactionReadCostController) evaluateMeasurement(owner *common.Owner, threshold int64, measurement *metrics.Measurement, at time.Time) (*alert.AlertEvaluationSnapshot, error) {
	// Check measurement identity and value.
	if measurement.Name != metrics.MetricTopicPartitions.Name ||
		measurement.Kind != metrics.MetricKindGauge ||
		measurement.Unit != metrics.MetricUnit(metrics.MetricTopicPartitions.Unit) {
		return alert.NewAlertEvaluationSnapshot(alert.AlertEvaluationStateInsufficientEvidence, nil, nil)
	}
	value := measurement.Value
	if math.IsNaN(value) || value < 0 || value >= math.MaxInt64 || math.Trunc(value) != value {
		return alert.NewAlertEvaluationSnapshot(alert.AlertEvaluationStateInsufficientEvidence, nil, nil)
	}

	// Read compaction applicability from the same observation.
	if len(measurement.Metadata) == 0 {
		return alert.NewAlertEvaluationSnapshot(alert.AlertEvaluationStateInsufficientEvidence, nil, nil)
	}
	var metadata metrics.PartitionMeasurementMetadata
	if err := json.Unmarshal(measurement.Metadata, &metadata); err != nil {
		return nil, err
	}

	// Uncompacted topics and counts below the threshold are healthy.
	switch metadata.CompactionStatus {
	case "uncompacted":
		return alert.NewAlertEvaluationSnapshot(alert.AlertEvaluationStateHealthy, nil, nil)
	case "compacted":
	default:
		return alert.NewAlertEvaluationSnapshot(alert.AlertEvaluationStateInsufficientEvidence, nil, nil)
	}
	count := int64(value)
	if count < threshold {
		return alert.NewAlertEvaluationSnapshot(alert.AlertEvaluationStateHealthy, nil, nil)
	}

	// Construct the active finding.
	finding, err := newCompactionReadCostAlert(owner, count, threshold, at)
	if err != nil {
		return nil, err
	}
	return alert.NewAlertEvaluationSnapshot(alert.AlertEvaluationStateActive, finding, nil)
}
