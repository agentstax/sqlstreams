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
	"github.com/agentstax/vulkan/pkg/metric"
	metricscontroller "github.com/agentstax/vulkan/pkg/metric/controller"
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
type MetricCollectorInstance struct {
	Owner  *common.Owner
	Config *MetricCollectorConfig
	Logger logging.Logger

	runner           *controller.InstanceTickRunner
	metrics          *metricscontroller.MetricController
	topics           *topiccontroller.TopicController
	alertHeads       *compactioncontroller.CompactionController
	metadata         *metricCollectorMetadata
	producerInstance *producer.ProducerInstance[metric.Measurement]
}

func newMetricCollectorInstance(collector *MetricCollectorProvisioner, owner *common.Owner, claimed *worker.WorkerInstance, metadata *metricCollectorMetadata, producerInstance *producer.ProducerInstance[metric.Measurement]) (*MetricCollectorInstance, error) {
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

	return &MetricCollectorInstance{
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
func (i *MetricCollectorInstance) Run(ctx context.Context) error {
	i.Logger.InfoContext(ctx, "metrics collector starting", "vulkan_version", common.BuildVersion(), "rate", i.metadata.PollRate)

	err := i.runner.Run(ctx, i.collect)
	if err == nil {
		i.Logger.InfoContext(ctx, "metrics collector stopped")
	}
	return err
}

// collect is one collection pass. A failed produce fails the whole pass --
// the next tick reproduces every measurement, so nothing is salvaged per measurement.
func (i *MetricCollectorInstance) collect(ctx context.Context) error {
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
	measurement, err := metric.NewBuiltInMeasurement(metric.MetricCollectorCompletedTimestamp, float64(at.Unix()), nil, at)
	if err != nil {
		return err
	}
	return i.produceMeasurement(ctx, measurement)
}

func (i *MetricCollectorInstance) collectWorkers(ctx context.Context, workers []metric.WorkerSnapshot) error {
	var unclaimed, failing int64
	var oldest time.Duration
	for _, snapshot := range workers {
		if snapshot.Status == metric.WorkerUnclaimed {
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
	measurement, err := metric.NewBuiltInMeasurement(metric.MetricUnclaimedWorkers, float64(unclaimed), nil, at)
	if err != nil {
		return err
	}
	if err := i.produceMeasurement(ctx, measurement); err != nil {
		return err
	}

	measurement, err = metric.NewBuiltInMeasurement(metric.MetricOldestUnclaimedAge, float64(oldest.Milliseconds()), nil, at)
	if err != nil {
		return err
	}
	if err := i.produceMeasurement(ctx, measurement); err != nil {
		return err
	}

	measurement, err = metric.NewBuiltInMeasurement(metric.MetricFailingWorkers, float64(failing), nil, at)
	if err != nil {
		return err
	}
	return i.produceMeasurement(ctx, measurement)
}

func (i *MetricCollectorInstance) collectSchedules(ctx context.Context) error {
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
	measurement, err := metric.NewBuiltInMeasurement(metric.MetricOverdueSchedules, float64(overdue), nil, at)
	if err != nil {
		return err
	}
	if err := i.produceMeasurement(ctx, measurement); err != nil {
		return err
	}

	measurement, err = metric.NewBuiltInMeasurement(metric.MetricOldestDueAge, float64(oldest.Milliseconds()), nil, at)
	if err != nil {
		return err
	}
	if err := i.produceMeasurement(ctx, measurement); err != nil {
		return err
	}

	measurement, err = metric.NewBuiltInMeasurement(metric.MetricSuspendedSchedules, float64(suspended), nil, at)
	if err != nil {
		return err
	}
	return i.produceMeasurement(ctx, measurement)
}

func (i *MetricCollectorInstance) collectAlerts(ctx context.Context) error {
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
	measurement, err := metric.NewBuiltInMeasurement(metric.MetricActiveAlerts, float64(active), nil, at)
	if err != nil {
		return err
	}
	if err := i.produceMeasurement(ctx, measurement); err != nil {
		return err
	}

	measurement, err = metric.NewBuiltInMeasurement(metric.MetricResolvedAlerts, float64(resolved), nil, at)
	if err != nil {
		return err
	}
	return i.produceMeasurement(ctx, measurement)
}

func (i *MetricCollectorInstance) collectTopics(ctx context.Context, workers []metric.WorkerSnapshot) error {
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

func (i *MetricCollectorInstance) collectTopic(ctx context.Context, current *topic.Topic, workers []metric.WorkerSnapshot) error {
	// Read the topic state and consumer-group measurements.
	snapshot, err := i.metrics.TopicSnapshot(ctx, current.Id)
	if err != nil {
		return err
	}

	at := time.Now()

	// Record partition count with compaction applicability for alert evaluation.
	partitionMetadata, err := metric.NewPartitionMeasurementMetadata(snapshot.Compacted)
	if err != nil {
		return err
	}

	measurement, err := metric.NewBuiltInMeasurement(metric.MetricTopicPartitions, float64(snapshot.Partitions), map[string]string{
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
	unclaimedWorkers := make([]*metric.UnclaimedWorkerMetadata, 0)
	for _, worker := range workers {
		if worker.Status != metric.WorkerUnclaimed {
			continue
		}
		metadata, err := metric.NewUnclaimedWorkerMetadata(worker.Name, worker.Owner, worker.TargetInstances)
		if err != nil {
			return err
		}
		unclaimedWorkers = append(unclaimedWorkers, metadata)
	}
	workerMetadata, err := metric.NewWorkerMeasurementMetadata(unclaimedWorkers)
	if err != nil {
		return err
	}

	measurement, err = metric.NewBuiltInMeasurement(metric.MetricTopicUnclaimedWorkers, float64(len(unclaimedWorkers)), map[string]string{
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
	if current.Name == metric.MetricTopicName {
		return nil
	}

	// Record compaction status as a scalar metric.
	compacted := float64(0)
	if snapshot.Compacted {
		compacted = 1
	}

	measurement, err = metric.NewBuiltInMeasurement(metric.MetricTopicCompacted, compacted, map[string]string{
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

func (i *MetricCollectorInstance) collectConsumerGroup(ctx context.Context, snapshot *metric.ConsumerGroupSnapshot, attributes map[string]string, at time.Time) error {
	points := []struct {
		metric *diagnostic.DiagnosticMetric
		value  float64
	}{
		{metric.MetricCursorHead, float64(snapshot.Cursor.Head)},
		{metric.MetricCursorClaimed, float64(snapshot.Cursor.Claimed)},
		{metric.MetricCursorCommitted, float64(snapshot.Cursor.Committed)},
		{metric.MetricCursorBacklog, float64(snapshot.Cursor.Backlog)},
		{metric.MetricCursorInflight, float64(snapshot.Cursor.Inflight)},
		{metric.MetricReadyExceptions, float64(snapshot.Exceptions.Ready)},
		{metric.MetricInflightExceptions, float64(snapshot.Exceptions.Inflight)},
		{metric.MetricDeferredExceptions, float64(snapshot.Exceptions.Deferred)},
		{metric.MetricDeadExceptions, float64(snapshot.Exceptions.Dead)},
		{metric.MetricOldestUnresolvedAge, float64(snapshot.Exceptions.OldestUnresolvedAge.Milliseconds())},
		{metric.MetricOpenLeases, float64(snapshot.OpenLeases)},
		{metric.MetricAbandonedOutstanding, float64(snapshot.AbandonedRoutines.Outstanding)},
		{metric.MetricAbandonedTotal, float64(snapshot.AbandonedRoutines.Total)},
		{metric.MetricAbandonedSelfClearLatencyAvg, float64(snapshot.AbandonedRoutines.SelfClearLatencyAvg.Milliseconds())},
	}

	items := make([]*producer.ProduceItem[metric.Measurement], 0, len(points))
	for _, point := range points {
		measurement, err := metric.NewBuiltInMeasurement(point.metric, point.value, attributes, at)
		if err != nil {
			return err
		}
		item, err := producer.NewProduceItem(measurement, &produce.ProduceOptions{
			RoutingKey: measurement.Name,
			MessageKey: metric.MeasurementKey(measurement.Name, measurement.Attributes),
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

func (i *MetricCollectorInstance) produceMeasurement(ctx context.Context, measurement *metric.Measurement) error {
	_, err := i.producerInstance.Produce(ctx, measurement, &produce.ProduceOptions{
		RoutingKey: measurement.Name,
		MessageKey: metric.MeasurementKey(measurement.Name, measurement.Attributes),
		Compaction: &produce.CompactionOptions{Enable: true},
	})
	return err
}

// ***************
// *** HELPERS ***
// ***************

func filterWorkersByTopic(workers []metric.WorkerSnapshot, topicId int64) []metric.WorkerSnapshot {
	topicWorkers := make([]metric.WorkerSnapshot, 0)
	for _, snapshot := range workers {
		if snapshot.Owner.TopicId == topicId {
			topicWorkers = append(topicWorkers, snapshot)
		}
	}
	return topicWorkers
}
