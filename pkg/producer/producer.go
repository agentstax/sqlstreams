package producer

import (
	"context"
	"errors"

	"github.com/allegedlyreliable/sqlstreams/pkg/alert"
	compactionreadcostcontroller "github.com/allegedlyreliable/sqlstreams/pkg/alert/compactionreadcost/controller"
	partitioncountcontroller "github.com/allegedlyreliable/sqlstreams/pkg/alert/partitioncount/controller"
	workerlivenesscontroller "github.com/allegedlyreliable/sqlstreams/pkg/alert/workerliveness/controller"
	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/common/logging"
	"github.com/allegedlyreliable/sqlstreams/pkg/datastore"
	"github.com/allegedlyreliable/sqlstreams/pkg/produce/controller"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
	streamcontroller "github.com/allegedlyreliable/sqlstreams/pkg/stream/controller"
)

type Producer struct {
	ds *datastore.PostgresDatastore
}

// NewProducer builds the datastore-only registration object. Register owns
// the config because each call returns an independently configured instance.
func NewProducer(ds *datastore.PostgresDatastore) (*Producer, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	return &Producer{ds: ds}, nil
}

// Register resolves the named stream against the live stream row and returns an
// instance that produces Message to it. Callable many times, with a
// different Message per call -- each call returns an independent instance.
// ctx bounds only this call's I/O.
func (p *Producer) Register[Message common.Versioned](ctx context.Context, streamName string, cfg *ProducerConfig) (*ProducerInstance[Message], error) {
	if streamName == "" {
		return nil, errors.New("stream name is required")
	}
	if cfg == nil {
		cfg = &ProducerConfig{}
	}
	cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	logger := logging.NewPipelineLogger(p.ds.Logger, &logging.PipelineLoggerConfig{Buffer: true, Suppress: true})

	produceController, err := controller.NewProduceController(p.ds, logger)
	if err != nil {
		return nil, err
	}
	streamController, err := streamcontroller.NewStreamController(p.ds, logger)
	if err != nil {
		return nil, err
	}
	partitionCountController, err := partitioncountcontroller.NewPartitionCountController(p.ds, logger)
	if err != nil {
		return nil, err
	}
	compactionReadCostController, err := compactionreadcostcontroller.NewCompactionReadCostController(p.ds, logger)
	if err != nil {
		return nil, err
	}
	workerLivenessController, err := workerlivenesscontroller.NewWorkerLivenessController(p.ds, logger)
	if err != nil {
		return nil, err
	}
	evaluators := []alert.Evaluator{partitionCountController, compactionReadCostController, workerLivenessController}

	current, err := streamController.Get(ctx, streamName)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, stream.ErrStreamNotFound.With("stream", streamName)
	}

	// fail fast if the db's schema is outside the range this build understands
	if err := streamController.AssertSchemaSupported(ctx, current.SystemId, current.Id); err != nil {
		return nil, err
	}

	p.logAlerts(ctx, current, logger, evaluators)

	return NewProducerInstance[Message](current, produceController, cfg, logger)
}
