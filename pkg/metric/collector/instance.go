package collector

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/alert"
	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/common/diagnostic"
	"github.com/allegedlyreliable/sqlstreams/pkg/common/logging"
	compactioncontroller "github.com/allegedlyreliable/sqlstreams/pkg/compaction/controller"
	"github.com/allegedlyreliable/sqlstreams/pkg/metric"
	metricscontroller "github.com/allegedlyreliable/sqlstreams/pkg/metric/controller"
	"github.com/allegedlyreliable/sqlstreams/pkg/produce"
	"github.com/allegedlyreliable/sqlstreams/pkg/producer"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
	streamcontroller "github.com/allegedlyreliable/sqlstreams/pkg/stream/controller"
	"github.com/allegedlyreliable/sqlstreams/pkg/worker"
	"github.com/allegedlyreliable/sqlstreams/pkg/worker/controller"
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
	streams          *streamcontroller.StreamController
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
		streams:          collector.streams,
		alertHeads:       collector.alertHeads,
		metadata:         metadata,
		producerInstance: producerInstance,
	}, nil
}

// Run collects until ctx cancels; a requested stop returns nil. The claimed
// instance releases on the way out however Run exits.
func (i *MetricCollectorInstance) Run(ctx context.Context) error {
	i.Logger.InfoContext(ctx, "metrics collector starting", "sqlstreams_version", common.BuildVersion(), "rate", i.metadata.PollRate)

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
	if err := i.collectStreams(ctx, workers); err != nil {
		return err
	}

	// Completion follows every collection write, including concurrent stream work.
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
	alertsStream, err := i.streams.Get(ctx, alert.AlertStreamName)
	if err != nil {
		return err
	}
	heads, err := i.alertHeads.ListHeads[alert.Alert](ctx, alertsStream.Id)
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

func (i *MetricCollectorInstance) collectStreams(ctx context.Context, workers []metric.WorkerSnapshot) error {
	streams, err := i.streams.List(ctx)
	if err != nil {
		return err
	}

	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(i.Config.StreamConcurrency)

	for _, current := range streams {
		group.Go(func() error {
			streamWorkers := filterWorkersByStream(workers, current.Id)
			return i.collectStream(groupCtx, current, streamWorkers)
		})
	}
	return group.Wait()
}

func (i *MetricCollectorInstance) collectStream(ctx context.Context, current *stream.Stream, workers []metric.WorkerSnapshot) error {
	// Read the stream state and consumer-group measurements.
	snapshot, err := i.metrics.StreamSnapshot(ctx, current.Id)
	if err != nil {
		return err
	}

	at := time.Now()

	// Record partition count with compaction applicability for alert evaluation.
	partitionMetadata, err := metric.NewPartitionMeasurementMetadata(snapshot.Compacted)
	if err != nil {
		return err
	}

	measurement, err := metric.NewBuiltInMeasurement(metric.MetricStreamPartitions, float64(snapshot.Partitions), map[string]string{
		"stream": current.Name,
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

	measurement, err = metric.NewBuiltInMeasurement(metric.MetricStreamUnclaimedWorkers, float64(len(unclaimedWorkers)), map[string]string{
		"stream": current.Name,
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
	if current.Name == metric.MetricStreamName {
		return nil
	}

	// Record compaction status as a scalar metric.
	compacted := float64(0)
	if snapshot.Compacted {
		compacted = 1
	}

	measurement, err = metric.NewBuiltInMeasurement(metric.MetricStreamCompacted, compacted, map[string]string{
		"stream": current.Name,
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
			"group":  group.ConsumerGroup,
			"stream": current.Name,
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

func filterWorkersByStream(workers []metric.WorkerSnapshot, streamId int64) []metric.WorkerSnapshot {
	streamWorkers := make([]metric.WorkerSnapshot, 0)
	for _, snapshot := range workers {
		if snapshot.Owner.StreamId == streamId {
			streamWorkers = append(streamWorkers, snapshot)
		}
	}
	return streamWorkers
}
