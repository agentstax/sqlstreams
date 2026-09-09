package janitor

import (
	"errors"

	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/common/logging"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	streamcontroller "github.com/agentstax/sqlstreams/pkg/stream/controller"
	janitorcontroller "github.com/agentstax/sqlstreams/pkg/stream/janitor/controller"
	"github.com/agentstax/sqlstreams/pkg/worker"
	"github.com/agentstax/sqlstreams/pkg/worker/controller"
)

const WorkerStreamJanitor = "stream_janitor"

type JanitorProvisioner struct {
	Config *JanitorConfig
	Logger logging.Logger

	workers    *controller.WorkerController
	streams    *streamcontroller.StreamController
	controller *janitorcontroller.JanitorController

	definition *worker.Definition
}

// cfg may be nil or sparse.
func NewJanitorProvisioner(ds *iDatastore.PostgresDatastore, cfg *JanitorConfig, logger logging.Logger) (*JanitorProvisioner, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if cfg == nil {
		cfg = &JanitorConfig{}
	}
	cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	workers, err := controller.NewWorkerController(ds, logger)
	if err != nil {
		return nil, err
	}

	streams, err := streamcontroller.NewStreamController(ds, logger)
	if err != nil {
		return nil, err
	}

	janitorController, err := janitorcontroller.NewJanitorController(ds, logger)
	if err != nil {
		return nil, err
	}

	definition, err := worker.NewDefinition(WorkerStreamJanitor, common.OwnerStream, 1, defaultJanitorMetadata())
	if err != nil {
		return nil, err
	}

	return &JanitorProvisioner{
		Config:     cfg,
		Logger:     logger,
		workers:    workers,
		streams:    streams,
		controller: janitorController,
		definition: definition,
	}, nil
}

func (d *JanitorProvisioner) Definition() *worker.Definition {
	return d.definition
}
