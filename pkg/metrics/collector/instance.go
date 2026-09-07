package collector

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/agentstax/vulkan/pkg/alert"
	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/common/diagnostic"
	"github.com/agentstax/vulkan/pkg/common/logging"
	compactioncontroller "github.com/agentstax/vulkan/pkg/compaction/controller"
	"github.com/agentstax/vulkan/pkg/metrics"
	metricscontroller "github.com/agentstax/vulkan/pkg/metrics/controller"
	"github.com/agentstax/vulkan/pkg/produce"
	"github.com/agentstax/vulkan/pkg/producer"
	"github.com/agentstax/vulkan/pkg/topic"
	topiccontroller "github.com/agentstax/vulkan/pkg/topic/controller"
	"github.com/agentstax/vulkan/pkg/worker"
	"github.com/agentstax/vulkan/pkg/worker/controller"
	"golang.org/x/sync/errgroup"
)

// reads the deployment's snapshots at the row's poll_rate while a heartbeat
// holds the claim, producing one Measurement per metric to __system.metrics
type MetricsCollectorInstance struct {
	Owner  *common.Owner
	Config *MetricsCollectorConfig
	Logger logging.Logger

	runner           *controller.InstanceTickRunner
	metrics          *metricscontroller.MetricsController
	topics           *topiccontroller.TopicController
	alertHeads       *compactioncontroller.CompactionController
	metadata         *metricsCollectorMetadata
	producerInstance *producer.ProducerInstance[metrics.Measurement]
}

func newMetricsCollectorInstance(collector *MetricsCollectorProvisioner, owner *common.Owner, claimed *worker.WorkerInstance, metadata *metricsCollectorMetadata, producerInstance *producer.ProducerInstance[metrics.Measurement]) (*MetricsCollectorInstance, error) {
	if owner == nil {
		return nil, errors.New("owner must not be nil")
	}
	if metadata == nil {
		return nil, errors.New("metadata must not be nil")
	}
	if producerInstance == nil {
		return nil, errors.New("producerInstance must not be nil")
	}

	logger := logging.NewPipelineLogger(collector.Logger, &logging.PipelineLoggerConfig{Args: []any{"worker", WorkerMetricsCollector, "system_id", owner.SystemId}})
	runner, err := controller.NewInstanceTickRunner(collector.workers, claimed, metadata.PollRate, &controller.InstanceTickRunnerConfig{
		InstanceTTL:    collector.Config.InstanceTTL,
		JitterFraction: collector.Config.JitterFraction,
		TickRetry:      collector.Config.CollectRetry,
	}, logger)
	if err != nil {
		return nil, err
	}

	return &MetricsCollectorInstance{
		Owner:            owner,
		Config:           collector.Config,
		Logger:           logger,
		runner:           runner,
		metrics:          collector.metrics,
		topics:           collector.topics,
		alertHeads:       collector.alertHeads,
		metadata:         metadata,
		producerInstance: producerInstance,
	}, nil
}

// Run collects until ctx cancels; a requested stop returns nil. The claimed
// instance releases on the way out however Run exits.
func (i *MetricsCollectorInstance) Run(ctx context.Context) error {
	i.Logger.InfoContext(ctx, "metrics collector starting", "vulkan_version", common.BuildVersion(), "rate", i.metadata.PollRate)

	err := i.runner.Run(ctx, i.collect)
	if err == nil {
		i.Logger.InfoContext(ctx, "metrics collector stopped")
	}
	return err
}

// collect is one collection pass. A failed produce fails the whole pass --
// the next tick reproduces every measurement, so nothing is salvaged per measurement.
func (i *MetricsCollectorInstance) collect(ctx context.Context) error {
	workers, err := i.metrics.WorkerSnapshots(ctx)
	if err != nil {
		return err
	}

	if err := i.collectWorkers(ctx, workers); err != nil {
		return err
	}
	if err := i.collectSchedules(ctx); err != nil {
		return err
	}
	if err := i.collectAlerts(ctx); err != nil {
		return err
	}
	if err := i.collectTopics(ctx, workers); err != nil {
		return err
	}

	// Completion follows every collection write, including concurrent topic work.
	at := time.Now()
	measurement, err := metrics.NewBuiltInMeasurement(metrics.MetricCollectorCompletedTimestamp, float64(at.Unix()), nil, at)
	if err != nil {
		return err
	}
	return i.produceMeasurement(ctx, measurement)
}

func (i *MetricsCollectorInstance) collectWorkers(ctx context.Context, workers []metrics.WorkerSnapshot) error {
	var unclaimed, failing int64
	var oldest time.Duration
	for _, snapshot := range workers {
		if snapshot.Status == metrics.WorkerUnclaimed {
			unclaimed++
			if snapshot.UnclaimedFor > oldest {
				oldest = snapshot.UnclaimedFor
			}
		}
		if snapshot.Attempts > 0 {
			failing++
		}
	}

	at := time.Now()
	measurement, err := metrics.NewBuiltInMeasurement(metrics.MetricUnclaimedWorkers, float64(unclaimed), nil, at)
	if err != nil {
		return err
	}
	if err := i.produceMeasurement(ctx, measurement); err != nil {
		return err
	}

	measurement, err = metrics.NewBuiltInMeasurement(metrics.MetricOldestUnclaimedAge, float64(oldest.Milliseconds()), nil, at)
	if err != nil {
		return err
	}
	if err := i.produceMeasurement(ctx, measurement); err != nil {
		return err
	}

	measurement, err = metrics.NewBuiltInMeasurement(metrics.MetricFailingWorkers, float64(failing), nil, at)
	if err != nil {
		return err
	}
	return i.produceMeasurement(ctx, measurement)
}

func (i *MetricsCollectorInstance) collectSchedules(ctx context.Context) error {
	schedules, err := i.metrics.ScheduleSnapshots(ctx)
	if err != nil {
		return err
	}

	var overdue, suspended int64
	var oldest time.Duration
	for _, found := range schedules {
		if found.Suspended {
			suspended++
			continue
		}
		if found.Overdue {
			overdue++
		}
		if found.DueFor > oldest {
			oldest = found.DueFor
		}
	}

	at := time.Now()
	measurement, err := metrics.NewBuiltInMeasurement(metrics.MetricOverdueSchedules, float64(overdue), nil, at)
	if err != nil {
		return err
	}
	if err := i.produceMeasurement(ctx, measurement); err != nil {
		return err
	}

	measurement, err = metrics.NewBuiltInMeasurement(metrics.MetricOldestDueAge, float64(oldest.Milliseconds()), nil, at)
	if err != nil {
		return err
	}
	if err := i.produceMeasurement(ctx, measurement); err != nil {
		return err
	}

	measurement, err = metrics.NewBuiltInMeasurement(metrics.MetricSuspendedSchedules, float64(suspended), nil, at)
	if err != nil {
		return err
	}
	return i.produceMeasurement(ctx, measurement)
}

func (i *MetricsCollectorInstance) collectAlerts(ctx context.Context) error {
	alertsTopic, err := i.topics.Get(ctx, alert.AlertTopicName)
	if err != nil {
		return err
	}
	heads, err := i.alertHeads.ListHeads[alert.Alert](ctx, alertsTopic.Id)
	if err != nil {
		return err
	}

	var active, resolved int64
	for _, head := range heads {
		switch head.Message.Status {
		case alert.AlertStatusActive:
			active++
		case alert.AlertStatusResolved:
			resolved++
		}
	}

	at := time.Now()
	measurement, err := metrics.NewBuiltInMeasurement(metrics.MetricActiveAlerts, float64(active), nil, at)
	if err != nil {
		return err
	}
	if err := i.produceMeasurement(ctx, measurement); err != nil {
		return err
	}

	measurement, err = metrics.NewBuiltInMeasurement(metrics.MetricResolvedAlerts, float64(resolved), nil, at)
	if err != nil {
		return err
	}
	return i.produceMeasurement(ctx, measurement)
}

func (i *MetricsCollectorInstance) collectTopics(ctx context.Context, workers []metrics.WorkerSnapshot) error {
	topics, err := i.topics.List(ctx)
	if err != nil {
		return err
	}

	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(i.Config.TopicConcurrency)

	for _, current := range topics {
		group.Go(func() error {
			topicWorkers := filterWorkersByTopic(workers, current.Id)
			return i.collectTopic(groupCtx, current, topicWorkers)
		})
	}
	return group.Wait()
}

func (i *MetricsCollectorInstance) collectTopic(ctx context.Context, current *topic.Topic, workers []metrics.WorkerSnapshot) error {
	// Read the topic state and consumer-group measurements.
	snapshot, err := i.metrics.TopicSnapshot(ctx, current.Id)
	if err != nil {
		return err
	}

	at := time.Now()

	// Record partition count with compaction applicability for alert evaluation.
	partitionMetadata, err := metrics.NewPartitionMeasurementMetadata(snapshot.Compacted)
	if err != nil {
		return err
	}

	measurement, err := metrics.NewBuiltInMeasurement(metrics.MetricTopicPartitions, float64(snapshot.Partitions), map[string]string{
		"topic": current.Name,
	}, at)
	if err != nil {
		return err
	}

	measurement.Metadata, err = json.Marshal(partitionMetadata)
	if err != nil {
		return err
	}

	if err := i.produceMeasurement(ctx, measurement); err != nil {
		return err
	}

	// Record unclaimed worker count and identities, including consumer-group workers.
	unclaimedWorkers := make([]*metrics.UnclaimedWorkerMetadata, 0)
	for _, worker := range workers {
		if worker.Status != metrics.WorkerUnclaimed {
			continue
		}
		metadata, err := metrics.NewUnclaimedWorkerMetadata(worker.Name, worker.Owner, worker.TargetInstances)
		if err != nil {
			return err
		}
		unclaimedWorkers = append(unclaimedWorkers, metadata)
	}
	workerMetadata, err := metrics.NewWorkerMeasurementMetadata(unclaimedWorkers)
	if err != nil {
		return err
	}

	measurement, err = metrics.NewBuiltInMeasurement(metrics.MetricTopicUnclaimedWorkers, float64(len(unclaimedWorkers)), map[string]string{
		"topic": current.Name,
	}, at)
	if err != nil {
		return err
	}

	measurement.Metadata, err = json.Marshal(workerMetadata)
	if err != nil {
		return err
	}

	if err := i.produceMeasurement(ctx, measurement); err != nil {
		return err
	}

	// Do not collect __system.metrics' own message traffic:
	// those writes would change what they measure.
	if current.Name == metrics.MetricsTopicName {
		return nil
	}

	// Record compaction status as a scalar metric.
	compacted := float64(0)
	if snapshot.Compacted {
		compacted = 1
	}

	measurement, err = metrics.NewBuiltInMeasurement(metrics.MetricTopicCompacted, compacted, map[string]string{
		"topic": current.Name,
	}, at)
	if err != nil {
		return err
	}

	if err := i.produceMeasurement(ctx, measurement); err != nil {
		return err
	}

	// Record each consumer group's cursor, exception, and lease measurements.
	for _, group := range snapshot.Groups {
		attributes := map[string]string{
			"group": group.ConsumerGroup,
			"topic": current.Name,
		}
		if err := i.collectConsumerGroup(ctx, &group, attributes, at); err != nil {
			return err
		}
	}
	return nil
}

func (i *MetricsCollectorInstance) collectConsumerGroup(ctx context.Context, snapshot *metrics.ConsumerGroupSnapshot, attributes map[string]string, at time.Time) error {
	points := []struct {
		metric *diagnostic.DiagnosticMetric
		value  float64
	}{
		{metrics.MetricCursorHead, float64(snapshot.Cursor.Head)},
		{metrics.MetricCursorClaimed, float64(snapshot.Cursor.Claimed)},
		{metrics.MetricCursorCommitted, float64(snapshot.Cursor.Committed)},
		{metrics.MetricCursorBacklog, float64(snapshot.Cursor.Backlog)},
		{metrics.MetricCursorInflight, float64(snapshot.Cursor.Inflight)},
		{metrics.MetricReadyExceptions, float64(snapshot.Exceptions.Ready)},
		{metrics.MetricInflightExceptions, float64(snapshot.Exceptions.Inflight)},
		{metrics.MetricDeferredExceptions, float64(snapshot.Exceptions.Deferred)},
		{metrics.MetricDeadExceptions, float64(snapshot.Exceptions.Dead)},
		{metrics.MetricOldestUnresolvedAge, float64(snapshot.Exceptions.OldestUnresolvedAge.Milliseconds())},
		{metrics.MetricOpenLeases, float64(snapshot.OpenLeases)},
		{metrics.MetricAbandonedOutstanding, float64(snapshot.AbandonedRoutines.Outstanding)},
		{metrics.MetricAbandonedTotal, float64(snapshot.AbandonedRoutines.Total)},
		{metrics.MetricAbandonedSelfClearLatencyAvg, float64(snapshot.AbandonedRoutines.SelfClearLatencyAvg.Milliseconds())},
	}

	items := make([]*producer.ProduceItem[metrics.Measurement], 0, len(points))
	for _, point := range points {
		measurement, err := metrics.NewBuiltInMeasurement(point.metric, point.value, attributes, at)
		if err != nil {
			return err
		}
		item, err := producer.NewProduceItem(measurement, &produce.ProduceOptions{
			RoutingKey: measurement.Name,
			MessageKey: metrics.MeasurementKey(measurement.Name, measurement.Attributes),
			Compaction: &produce.CompactionOptions{Enable: true},
		})
		if err != nil {
			return err
		}
		items = append(items, item)
	}

	_, err := i.producerInstance.ProduceBatch(ctx, items...)
	return err
}

func (i *MetricsCollectorInstance) produceMeasurement(ctx context.Context, measurement *metrics.Measurement) error {
	_, err := i.producerInstance.Produce(ctx, measurement, &produce.ProduceOptions{
		RoutingKey: measurement.Name,
		MessageKey: metrics.MeasurementKey(measurement.Name, measurement.Attributes),
		Compaction: &produce.CompactionOptions{Enable: true},
	})
	return err
}

// ***************
// *** HELPERS ***
// ***************

func filterWorkersByTopic(workers []metrics.WorkerSnapshot, topicId int64) []metrics.WorkerSnapshot {
	topicWorkers := make([]metrics.WorkerSnapshot, 0)
	for _, snapshot := range workers {
		if snapshot.Owner.TopicId == topicId {
			topicWorkers = append(topicWorkers, snapshot)
		}
	}
	return topicWorkers
}
