package janitor

import (
	"errors"
	"time"

	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/common/logging"
	janitorcontroller "github.com/agentstax/sqlstreams/pkg/consume/janitor/controller"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	"github.com/agentstax/sqlstreams/pkg/worker"
	"github.com/agentstax/sqlstreams/pkg/worker/controller"
)

const WorkerConsumerGroupJanitor = "consumer_group_janitor"

// waitingDeclarationTTL is how long a superseded waiting binding_log row
// stays before the sweep deletes it. Each declarer's newest waiting row is
// kept regardless, so a dead waiter stays visible in listings.
const waitingDeclarationTTL = 7 * 24 * time.Hour

type JanitorProvisioner struct {
	Config *JanitorConfig
	Logger logging.Logger

	workers    *controller.WorkerController
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

	sweepController, err := janitorcontroller.NewJanitorController(ds, logger)
	if err != nil {
		return nil, err
	}

	definition, err := worker.NewDefinition(WorkerConsumerGroupJanitor, common.OwnerSystem, 1, defaultJanitorMetadata())
	if err != nil {
		return nil, err
	}

	return &JanitorProvisioner{
		Config:     cfg,
		Logger:     logger,
		workers:    workers,
		controller: sweepController,
		definition: definition,
	}, nil
}

func (d *JanitorProvisioner) Definition() *worker.Definition {
	return d.definition
}
