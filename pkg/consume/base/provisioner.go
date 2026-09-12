package base

// Package base holds the pieces every consumer worker row shares. It
// assembles nothing: consumer.NewConsumer is the assembled group, a
// sub-consumer provisioner runs one worker row.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/common/logging"
	"github.com/allegedlyreliable/sqlstreams/pkg/consume/base/controller"
	"github.com/allegedlyreliable/sqlstreams/pkg/datastore"
	metricsproducer "github.com/allegedlyreliable/sqlstreams/pkg/metric/producer"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
	streamcontroller "github.com/allegedlyreliable/sqlstreams/pkg/stream/controller"
	"github.com/allegedlyreliable/sqlstreams/pkg/worker"
	workercontroller "github.com/allegedlyreliable/sqlstreams/pkg/worker/controller"
)

// BaseProvisioner is the half of a consumer worker kind every row shares:
// the row's definition, the controllers, and the group's consumerFunc.
type BaseProvisioner[Message common.Versioned] struct {
	definition *worker.Definition
	Logger     logging.Logger

	workers      *workercontroller.WorkerController
	streams      *streamcontroller.StreamController
	keyLeases    *controller.KeyLeaseController
	metrics      *metricsproducer.MetricProducer
	consumerFunc func(ctx context.Context, message *Message) error

	// the version the group's Message type declares; the claim reads only rows at it
	schemaVersion int
}

func NewBaseProvisioner[Message common.Versioned](ds *datastore.PostgresDatastore, definition *worker.Definition, consumerFunc func(ctx context.Context, message *Message) error, schemaVersion int, metrics *metricsproducer.MetricProducer, logger logging.Logger) (*BaseProvisioner[Message], error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if definition == nil {
		return nil, errors.New("definition must not be nil")
	}
	if consumerFunc == nil {
		return nil, errors.New("consumerFunc must not be nil")
	}
	if schemaVersion < 1 {
		return nil, fmt.Errorf("schemaVersion must be >= 1, got %d", schemaVersion)
	}
	if metrics == nil {
		return nil, errors.New("metrics must not be nil")
	}
	if definition.TargetInstances != worker.NoInstanceTarget {
		return nil, fmt.Errorf("definition.TargetInstances must be %d for a consumer, got %d", worker.NoInstanceTarget, definition.TargetInstances)
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	workers, err := workercontroller.NewWorkerController(ds, logger)
	if err != nil {
		return nil, err
	}

	streams, err := streamcontroller.NewStreamController(ds, logger)
	if err != nil {
		return nil, err
	}

	keyLeases, err := controller.NewKeyLeaseController(ds, logger)
	if err != nil {
		return nil, err
	}

	return &BaseProvisioner[Message]{
		definition:    definition,
		Logger:        logger,
		workers:       workers,
		streams:       streams,
		keyLeases:     keyLeases,
		metrics:       metrics,
		consumerFunc:  consumerFunc,
		schemaVersion: schemaVersion,
	}, nil
}

func (d *BaseProvisioner[Message]) Definition() *worker.Definition {
	return d.definition
}

// Declare writes the definition as the owner group's worker row -- the
// newest declaration wins.
func (d *BaseProvisioner[Message]) Declare(ctx context.Context, owner *common.Owner) error {
	return d.workers.DeclareWorker(ctx, d.definition, owner)
}

// GetStream resolves the stream a consumer's owner points at; a missing stream
// is an error, not an expected absence -- nothing can consume from it.
func (d *BaseProvisioner[Message]) GetStream(ctx context.Context, streamId int64) (*stream.Stream, error) {
	current, err := d.streams.GetById(ctx, streamId)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, stream.ErrStreamNotFound.With("stream_id", streamId)
	}
	return current, nil
}

// RegisterInstance claims one live instance under the worker row; a nil
// instance is a declined claim, not an error.
func (d *BaseProvisioner[Message]) RegisterInstance(ctx context.Context, workerId int64, owner *common.Owner, instanceTTL time.Duration) (*worker.WorkerInstance, error) {
	return d.workers.RegisterInstance(ctx, workerId, owner, d.definition.OwnerKind, d.definition.Name, instanceTTL)
}
