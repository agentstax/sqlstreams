package cursoradvancer

import (
	"errors"

	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/common/logging"
	cursoradvancercontroller "github.com/agentstax/sqlstreams/pkg/consume/cursoradvancer/controller"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	"github.com/agentstax/sqlstreams/pkg/worker"
	"github.com/agentstax/sqlstreams/pkg/worker/controller"
)

const WorkerCursorAdvancer = "cursor_advancer"

type CursorAdvancerProvisioner struct {
	Config *CursorAdvancerConfig
	Logger logging.Logger

	workers    *controller.WorkerController
	controller *cursoradvancercontroller.CursorAdvancerController

	definition *worker.Definition
}

// cfg may be nil or sparse.
func NewCursorAdvancerProvisioner(ds *iDatastore.PostgresDatastore, cfg *CursorAdvancerConfig, logger logging.Logger) (*CursorAdvancerProvisioner, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if cfg == nil {
		cfg = &CursorAdvancerConfig{}
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

	advanceController, err := cursoradvancercontroller.NewCursorAdvancerController(ds, logger)
	if err != nil {
		return nil, err
	}

	definition, err := worker.NewDefinition(WorkerCursorAdvancer, common.OwnerConsumerGroup, 1, defaultCursorAdvancerMetadata())
	if err != nil {
		return nil, err
	}

	return &CursorAdvancerProvisioner{
		Config:     cfg,
		Logger:     logger,
		workers:    workers,
		controller: advanceController,
		definition: definition,
	}, nil
}

func (d *CursorAdvancerProvisioner) Definition() *worker.Definition {
	return d.definition
}
