package partitioncount

import (
	"context"
	"errors"
	"time"

	"github.com/agentstax/sqlstreams/pkg/alert"
	alertcontroller "github.com/agentstax/sqlstreams/pkg/alert/controller"
	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/common/diagnostic"
	"github.com/agentstax/sqlstreams/pkg/common/logging"
	"github.com/agentstax/sqlstreams/pkg/consumer"
	"github.com/agentstax/sqlstreams/pkg/metric"
	"github.com/agentstax/sqlstreams/pkg/produce"
	"github.com/agentstax/sqlstreams/pkg/producer"
	"github.com/agentstax/sqlstreams/pkg/schedule"
	"github.com/agentstax/sqlstreams/pkg/worker"
	workercontroller "github.com/agentstax/sqlstreams/pkg/worker/controller"
)

// PartitionCountInstance consumes the alert's schedule messages while a heartbeat
// holds the claim.
type PartitionCountInstance struct {
	Owner  *common.Owner
	Logger logging.Logger

	provisioner    *PartitionCountProvisioner
	runner         *workercontroller.InstanceRunner
	repeatInterval time.Duration
	alerts         *alertcontroller.AlertController // built per claimed life in consume
	measurements   *producer.ProducerInstance[metric.Measurement]
}

func newPartitionCountInstance(provisioner *PartitionCountProvisioner, owner *common.Owner, claimed *worker.WorkerInstance, repeatInterval time.Duration) (*PartitionCountInstance, error) {
	if owner == nil {
		return nil, errors.New("owner must not be nil")
	}

	logger := logging.NewPipelineLogger(provisioner.Logger, &logging.PipelineLoggerConfig{Args: []any{"worker", JobName, "group", owner.Name}})
	runner, err := workercontroller.NewInstanceRunner(provisioner.workers, claimed, &workercontroller.InstanceRunnerConfig{
		InstanceTTL: provisioner.Config.InstanceTTL,
	}, logger)
	if err != nil {
		return nil, err
	}

	return &PartitionCountInstance{
		Owner:          owner,
		Logger:         logger,
		provisioner:    provisioner,
		runner:         runner,
		repeatInterval: repeatInterval,
	}, nil
}

// Run consumes until ctx cancels; a requested stop returns nil. The claimed
// instance releases on the way out however Run exits.
func (i *PartitionCountInstance) Run(ctx context.Context) error {
	return i.runner.Run(ctx, i.consume)
}

// consume is one claimed life: the alert controller is built here so every
// claim applies the claimed row's repeat_interval against the alerts stream's
// live retention.
func (i *PartitionCountInstance) consume(ctx context.Context) error {
	registered, err := i.provisioner.producer.Register[alert.Alert](ctx, alert.AlertStreamName, nil)
	if err != nil {
		return err
	}
	measurements, err := i.provisioner.producer.Register[metric.Measurement](ctx, metric.MetricStreamName, nil)
	if err != nil {
		return err
	}
	i.measurements = measurements

	alerts, err := alertcontroller.NewAlertController(ctx, registered, i.provisioner.ds, i.provisioner.alertHeads, i.repeatInterval, i.Logger)
	if err != nil {
		return err
	}
	i.alerts = alerts

	instance, err := i.provisioner.scheduleConsumer.Register[alert.JobPayload](ctx, JobName, schedule.ScheduleStreamName, &consumer.ConsumerConfig{
		Bindings: []string{JobName},
	})
	if err != nil {
		return err
	}
	return instance.Consume(ctx, i.evaluateStreams, nil)
}

func (i *PartitionCountInstance) evaluateStreams(ctx context.Context, jobPayload *alert.JobPayload) error {
	streams, err := i.provisioner.streams.List(ctx)
	if err != nil {
		return err
	}

	// one stream's failure never skips the others
	var evaluated, failed, published, resolved int64
	var errs error
	for _, listed := range streams {
		evaluated++
		owner, err := common.NewStreamOwner(listed.SystemId, listed.Id, listed.Name)
		if err != nil {
			failed++
			errs = errors.Join(errs, err)
			continue
		}

		result, err := i.provisioner.controller.Evaluate(ctx, owner, jobPayload)
		if err != nil {
			failed++
			errs = errors.Join(errs, err)
			continue
		}

		outcome, err := i.alerts.Record(ctx, alert.AlertPartitionCount.Name, owner, result)
		if err != nil {
			failed++
			errs = errors.Join(errs, err)
			continue
		}
		switch outcome {
		case alert.RecordOutcomeNothing:
			if result.State == alert.AlertEvaluationStateInsufficientEvidence {
				failed++
			}
		case alert.RecordOutcomeActive:
			published++
		case alert.RecordOutcomeResolved:
			resolved++
		}
	}

	// the summary goes out even on a failed run
	err = i.produceCheckSummary(ctx, evaluated, failed, published, resolved)
	return errors.Join(errs, err)
}

func (i *PartitionCountInstance) produceCheckSummary(ctx context.Context, evaluated int64, failed int64, published int64, resolved int64) error {
	attributes := map[string]string{"alert": alert.AlertPartitionCount.Name}
	at := time.Now()

	counts := []struct {
		metric *diagnostic.DiagnosticMetric
		value  int64
	}{
		{metric.MetricCheckStreamsEvaluated, evaluated},
		{metric.MetricCheckStreamsFailed, failed},
		{metric.MetricCheckPublishedAlerts, published},
		{metric.MetricCheckResolvedAlerts, resolved},
	}

	items := make([]*producer.ProduceItem[metric.Measurement], 0, len(counts))
	for _, count := range counts {
		measurement, err := metric.NewBuiltInMeasurement(count.metric, float64(count.value), attributes, at)
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

	_, err := i.measurements.ProduceBatch(ctx, items...)
	return err
}
