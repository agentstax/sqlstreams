package collectorprogress

import (
	"context"
	"errors"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/alert"
	alertcontroller "github.com/allegedlyreliable/sqlstreams/pkg/alert/controller"
	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/common/logging"
	"github.com/allegedlyreliable/sqlstreams/pkg/consumer"
	"github.com/allegedlyreliable/sqlstreams/pkg/schedule"
	"github.com/allegedlyreliable/sqlstreams/pkg/worker"
	workercontroller "github.com/allegedlyreliable/sqlstreams/pkg/worker/controller"
)

// CollectorProgressInstance consumes scheduled checks while a heartbeat holds its claim.
type CollectorProgressInstance struct {
	Owner  *common.Owner
	Logger logging.Logger

	provisioner    *CollectorProgressProvisioner
	runner         *workercontroller.InstanceRunner
	repeatInterval time.Duration
	alerts         *alertcontroller.AlertController
}

func newCollectorProgressInstance(provisioner *CollectorProgressProvisioner, owner *common.Owner, claimed *worker.WorkerInstance, repeatInterval time.Duration) (*CollectorProgressInstance, error) {
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

	return &CollectorProgressInstance{
		Owner:          owner,
		Logger:         logger,
		provisioner:    provisioner,
		runner:         runner,
		repeatInterval: repeatInterval,
	}, nil
}

// Run consumes checks until ctx cancels or the claim is lost.
func (i *CollectorProgressInstance) Run(ctx context.Context) error {
	return i.runner.Run(ctx, i.consume)
}

func (i *CollectorProgressInstance) consume(ctx context.Context) error {
	registered, err := i.provisioner.producer.Register[alert.Alert](ctx, alert.AlertStreamName, nil)
	if err != nil {
		return err
	}
	alerts, err := alertcontroller.NewAlertController(ctx, registered, i.provisioner.ds, i.provisioner.alertHeads, i.repeatInterval, i.Logger)
	if err != nil {
		return err
	}
	i.alerts = alerts

	instance, err := i.provisioner.scheduleConsumer.Register[alert.JobPayload](ctx, JobName, schedule.ScheduleStreamName, &consumer.ConsumerConfig{Bindings: []string{JobName}})
	if err != nil {
		return err
	}
	return instance.Consume(ctx, i.evaluateSystem, nil)
}

func (i *CollectorProgressInstance) evaluateSystem(ctx context.Context, payload *alert.JobPayload) error {
	owner, err := common.NewSystemOwner(i.Owner.SystemId)
	if err != nil {
		return err
	}
	result, err := i.provisioner.controller.Evaluate(ctx, owner, payload)
	if err != nil {
		return err
	}
	if result.State == alert.AlertEvaluationStateInsufficientEvidence {
		i.Logger.WarnContext(ctx, "collector progress evidence is insufficient -- recorded alert unchanged")
	}
	_, err = i.alerts.Record(ctx, alert.AlertMetricsCollectorProgress.Name, owner, result)
	return err
}
