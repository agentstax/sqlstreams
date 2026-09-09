package messageconsumer

import (
	"context"

	consumebase "github.com/agentstax/sqlstreams/pkg/consume/base"
	"github.com/agentstax/sqlstreams/pkg/worker"
	workercontroller "github.com/agentstax/sqlstreams/pkg/worker/controller"
)

// a nil Execution is a declined claim, not an error -- try again later.
func (d *MessageConsumerProvisioner[Message]) Provision(ctx context.Context, declared *worker.Worker) (worker.Execution, error) {
	parsed, err := workercontroller.ParseMetadata[MessageConsumerMetadata](declared.Metadata)
	if err != nil {
		return nil, err
	}
	if err := parsed.Validate(); err != nil {
		return nil, err
	}
	claimed, err := d.RegisterInstance(ctx, declared.Id, declared.Owner, d.Config.InstanceTTL)
	if err != nil || claimed == nil {
		return nil, err
	}

	cfg := d.Config.withMetadata(parsed)
	resolvedStream, err := d.GetStream(ctx, declared.Owner.StreamId)
	if err != nil {
		return nil, err
	}

	base, err := consumebase.NewBaseConsumer(d.BaseProvisioner, declared.Owner, resolvedStream, &consumebase.BaseConsumerConfig{
		TimeoutGrace:          cfg.TimeoutGrace,
		RecordMargin:          cfg.RecordMargin,
		SlowDispatchThreshold: cfg.SlowDispatchThreshold,
	})
	if err != nil {
		return nil, err
	}

	runner, err := newMessageRunner(base, d.consumers, cfg, parsed)
	if err != nil {
		return nil, err
	}

	return consumebase.NewBaseInstance(d.BaseProvisioner, declared.Owner, claimed, cfg.InstanceTTL, runner.run)
}
