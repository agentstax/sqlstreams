package consumer

import (
	"context"

	"github.com/agentstax/sqlstreams/pkg/consume/cursoradvancer"
	"github.com/agentstax/sqlstreams/pkg/consume/exceptionconsumer"
	consumejanitor "github.com/agentstax/sqlstreams/pkg/consume/janitor"
	"github.com/agentstax/sqlstreams/pkg/consume/messageconsumer"
	"github.com/agentstax/sqlstreams/pkg/metric/collector"
	scheduleproducer "github.com/agentstax/sqlstreams/pkg/schedule/producer"
	streamjanitor "github.com/agentstax/sqlstreams/pkg/stream/janitor"
	"github.com/agentstax/sqlstreams/pkg/stream/vacuum"
	"github.com/agentstax/sqlstreams/pkg/worker"
	"github.com/agentstax/sqlstreams/pkg/worker/manager"
)

// the group's consumer rows and their config document are declared by
// Register; the upkeep rows below are declared here, so a second Consume
// re-creates whatever a crash lost
func (i *ConsumerInstance[Message]) newManagerRunner(ctx context.Context, consumerFunc ConsumerFunc[Message], options *ConsumeOptions) (*manager.Runner, error) {
	groupProvisioners, err := i.newGroupProvisioners(ctx, consumerFunc, options)
	if err != nil {
		return nil, err
	}
	streamProvisioners, err := i.newStreamProvisioners()
	if err != nil {
		return nil, err
	}

	provisioners := make([]worker.Provisioner, 0, len(groupProvisioners)+len(streamProvisioners))
	provisioners = append(provisioners, groupProvisioners...)
	provisioners = append(provisioners, streamProvisioners...)

	managerProvisioner, err := manager.NewManagerProvisioner(i.ds, worker.NoInstanceTarget, nil, i.Logger, provisioners...)
	if err != nil {
		return nil, err
	}

	if err := managerProvisioner.Declare(ctx, i.Owner); err != nil {
		return nil, err
	}

	return manager.NewRunner(managerProvisioner, i.Owner, nil, i.Logger)
}

// one frontier per group, with committed advancing behind it. Each
// provisioner declares its own row before it joins the manager's list.
func (i *ConsumerInstance[Message]) newGroupProvisioners(ctx context.Context, consumerFunc ConsumerFunc[Message], options *ConsumeOptions) ([]worker.Provisioner, error) {
	message, err := messageconsumer.NewMessageConsumerProvisioner(i.ds, consumerFunc, i.streamVersion, i.metrics, toMessageConsumerConfig(i.Config, options), i.Logger)
	if err != nil {
		return nil, err
	}

	exception, err := exceptionconsumer.NewExceptionConsumerProvisioner(i.ds, consumerFunc, i.streamVersion, i.metrics, toExceptionConsumerConfig(i.Config, options), i.Logger)
	if err != nil {
		return nil, err
	}

	cursorAdvancerProvisioner, err := cursoradvancer.NewCursorAdvancerProvisioner(i.ds, nil, i.Logger)
	if err != nil {
		return nil, err
	}
	if err := cursorAdvancerProvisioner.Declare(ctx, i.Owner); err != nil {
		return nil, err
	}

	return []worker.Provisioner{message, exception, cursorAdvancerProvisioner}, nil
}

func (i *ConsumerInstance[Message]) newStreamProvisioners() ([]worker.Provisioner, error) {
	streamJanitorProvisioner, err := streamjanitor.NewJanitorProvisioner(i.ds, nil, i.Logger)
	if err != nil {
		return nil, err
	}

	streamVacuumProvisioner, err := vacuum.NewVacuumProvisioner(i.ds, nil, i.Logger)
	if err != nil {
		return nil, err
	}

	consumerGroupJanitorProvisioner, err := consumejanitor.NewJanitorProvisioner(i.ds, nil, i.Logger)
	if err != nil {
		return nil, err
	}

	scheduleProducerProvisioner, err := scheduleproducer.NewScheduleProducerProvisioner(i.ds, nil, i.Logger)
	if err != nil {
		return nil, err
	}

	metricCollectorProvisioner, err := collector.NewMetricsCollectorProvisioner(i.ds, nil, i.Logger)
	if err != nil {
		return nil, err
	}

	return []worker.Provisioner{scheduleProducerProvisioner, metricCollectorProvisioner, streamJanitorProvisioner, streamVacuumProvisioner, consumerGroupJanitorProvisioner}, nil
}
