package controller

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/agentstax/vulkan/pkg/alert"
	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/metrics"
	"github.com/agentstax/vulkan/pkg/metrics/collector"
	"github.com/agentstax/vulkan/pkg/worker"
	workercontroller "github.com/agentstax/vulkan/pkg/worker/controller"
	"github.com/agentstax/vulkan/pkg/worker/manager"
)

// Evaluate compares the latest completion with continuous system-manager lease coverage.
// No measurement or alert is written; MaximumAge 0 uses the collector's declared poll rate.
func (c *CollectorProgressController) Evaluate(ctx context.Context, owner *common.Owner, policy *alert.JobPayload) (*alert.AlertEvaluationResult, error) {
	if err := workercontroller.ValidateOwner(owner, common.OwnerSystem, alert.AlertMetricsCollectorProgress.Name); err != nil {
		return nil, err
	}
	if policy == nil {
		return nil, errors.New("policy must not be nil")
	}
	if err := policy.WithDefaults().Validate(); err != nil {
		return nil, err
	}

	maximumAge, err := c.maximumAge(ctx, owner, policy.MaximumAge)
	if err != nil {
		return nil, err
	}
	completion, err := c.metrics.GetMeasurement(ctx, metrics.MetricCollectorCompletedTimestamp.Name)
	if err != nil {
		return nil, err
	}
	declared, err := c.workers.GetWorker(ctx, manager.WorkerManager, owner)
	if err != nil {
		return nil, err
	}
	history, err := c.workers.GetInstanceHistory(ctx, declared.Id, policy.PendingDuration)
	if err != nil {
		return nil, err
	}
	return c.evaluateHistory(owner, completion, history, maximumAge, policy)
}

func (c *CollectorProgressController) maximumAge(ctx context.Context, owner *common.Owner, configured time.Duration) (time.Duration, error) {
	if configured > 0 {
		return configured, nil
	}
	declared, err := c.workers.GetWorker(ctx, collector.WorkerMetricsCollector, owner)
	if err != nil {
		return 0, err
	}
	metadata, err := workercontroller.ParseMetadata[map[string]time.Duration](declared.Metadata)
	if err != nil {
		return 0, err
	}
	pollRate := (*metadata)["poll_rate"]
	if pollRate <= 0 || pollRate > time.Duration(math.MaxInt64)/3 {
		return 0, fmt.Errorf("poll_rate must be in (0, %v], got %v", time.Duration(math.MaxInt64)/3, pollRate)
	}
	return max(2*time.Minute, 3*pollRate), nil
}

func (c *CollectorProgressController) evaluateHistory(owner *common.Owner, completion *common.StoredMessage[metrics.Measurement], history *worker.WorkerInstanceHistory, maximumAge time.Duration, policy *alert.JobPayload) (*alert.AlertEvaluationResult, error) {
	current := history.EvaluatedAt
	completedAt, usable := completionTimestamp(completion, current)
	if !usable {
		return alert.NewAlertEvaluationResult(alert.AlertEvaluationStateInsufficientEvidence, nil)
	}
	if !completedAt.IsZero() && current.Sub(completedAt) < maximumAge {
		return alert.NewAlertEvaluationResult(alert.AlertEvaluationStateHealthy, nil)
	}

	// Without current manager coverage, no unhealthy duration can be established.
	managerCoverageStart := continuousLeaseStart(history)
	if managerCoverageStart.IsZero() {
		return alert.NewAlertEvaluationResult(alert.AlertEvaluationStateInsufficientEvidence, nil)
	}

	// Time before continuous manager coverage, including a shutdown, does not count.
	unhealthySince := managerCoverageStart

	// Only count history inside the window requested for this evaluation.
	historyWindowStart := current.Add(-policy.PendingDuration)
	if historyWindowStart.After(unhealthySince) {
		unhealthySince = historyWindowStart
	}

	// A completed pass stays healthy until its maximum age; that time does not count.
	// Without a completion, count from the coverage/window start above.
	if !completedAt.IsZero() {
		completionOverdueAt := completedAt.Add(maximumAge)
		if completionOverdueAt.After(unhealthySince) {
			unhealthySince = completionOverdueAt
		}
	}
	unhealthyDuration := current.Sub(unhealthySince)

	finding, err := newCollectorProgressAlert(owner, completedAt, maximumAge, current)
	if err != nil {
		return nil, err
	}
	if policy.DisablePending || unhealthyDuration >= policy.PendingDuration {
		return alert.NewAlertEvaluationResult(alert.AlertEvaluationStateActive, finding)
	}
	return alert.NewAlertEvaluationResult(alert.AlertEvaluationStatePending, finding)
}

// ***************
// *** HELPERS ***
// ***************

// Missing completion is usable absence; malformed or future evidence is not.
func completionTimestamp(completion *common.StoredMessage[metrics.Measurement], current time.Time) (time.Time, bool) {
	if completion == nil {
		return time.Time{}, true
	}
	measurement := completion.Message
	if measurement.Name != metrics.MetricCollectorCompletedTimestamp.Name ||
		measurement.Kind != metrics.MetricKindGauge ||
		measurement.Unit != metrics.MetricUnit(metrics.MetricCollectorCompletedTimestamp.Unit) {
		return time.Time{}, false
	}
	value := measurement.Value
	if math.IsNaN(value) || value <= 0 || value >= math.MaxInt64 || math.Trunc(value) != value {
		return time.Time{}, false
	}
	completedAt := time.Unix(int64(value), 0)
	if completedAt.After(current) || completion.CreatedAt.After(current) {
		return time.Time{}, false
	}
	return completedAt, true
}

// Instances are ordered by creation time descending; touching leases preserve coverage.
func continuousLeaseStart(history *worker.WorkerInstanceHistory) time.Time {
	var startedAt time.Time
	for _, instance := range history.Instances {
		if startedAt.IsZero() {
			if instance.ExpiresAt.After(history.EvaluatedAt) {
				startedAt = instance.CreatedAt
			}
			continue
		}
		if instance.ExpiresAt.Before(startedAt) {
			continue
		}
		startedAt = instance.CreatedAt
	}
	return startedAt
}
