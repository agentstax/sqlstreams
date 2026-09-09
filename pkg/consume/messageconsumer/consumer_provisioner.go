package messageconsumer

// Package messageconsumer is ONE worker row of a consumer group: the loop
// that claims and processes fresh messages. consumer.NewConsumer assembles
// the full group; applications consume through it.
//
// Run alone:
//   - exception rows are written but never retried
//   - committed never advances, pinning retention
//   - the unresolved-exceptions alert eventually surfaces both

import (
	"context"
	"errors"

	"github.com/agentstax/sqlstreams/pkg/common/logging"

	"github.com/agentstax/sqlstreams/pkg/common"
	consumebase "github.com/agentstax/sqlstreams/pkg/consume/base"
	"github.com/agentstax/sqlstreams/pkg/consume/messageconsumer/controller"
	"github.com/agentstax/sqlstreams/pkg/datastore"
	metricsproducer "github.com/agentstax/sqlstreams/pkg/metric/producer"
	"github.com/agentstax/sqlstreams/pkg/worker"
)

// setting this row's target_instances to 0 suspends just this kind's new
// claims, leaving the group's other consumer rows running
const WorkerMessageConsumer = "message_consumer"

type MessageConsumerProvisioner[Message common.Versioned] struct {
	Config *MessageConsumerConfig

	*consumebase.BaseProvisioner[Message]

	consumers *controller.MessageConsumerGroupController
}

// NewMessageConsumerProvisioner builds one worker row of the group, not the
// assembled consumer -- see the package doc.
// cfg may be nil or sparse.
func NewMessageConsumerProvisioner[Message common.Versioned](ds *datastore.PostgresDatastore, consumerFunc func(ctx context.Context, message *Message) error, schemaVersion int, metrics *metricsproducer.MetricProducer, cfg *MessageConsumerConfig, logger logging.Logger) (*MessageConsumerProvisioner[Message], error) {
	if cfg == nil {
		cfg = &MessageConsumerConfig{}
	}
	cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	definition, err := worker.NewDefinition(WorkerMessageConsumer, common.OwnerConsumerGroup, worker.NoInstanceTarget, toMessageConsumerMetadata(cfg))
	if err != nil {
		return nil, err
	}
	baseProvisioner, err := consumebase.NewBaseProvisioner(ds, definition, consumerFunc, schemaVersion, metrics, logger)
	if err != nil {
		return nil, err
	}
	consumers, err := controller.NewMessageConsumerGroupController(ds, logger)
	if err != nil {
		return nil, err
	}

	return &MessageConsumerProvisioner[Message]{
		Config:          cfg,
		BaseProvisioner: baseProvisioner,
		consumers:       consumers,
	}, nil
}
