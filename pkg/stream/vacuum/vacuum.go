package vacuum

import (
	"errors"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/common/logging"
	iDatastore "github.com/allegedlyreliable/sqlstreams/pkg/datastore"
	streamcontroller "github.com/allegedlyreliable/sqlstreams/pkg/stream/controller"
	vacuumcontroller "github.com/allegedlyreliable/sqlstreams/pkg/stream/vacuum/controller"
	"github.com/allegedlyreliable/sqlstreams/pkg/worker"
	"github.com/allegedlyreliable/sqlstreams/pkg/worker/controller"
)

const WorkerStreamVacuum = "stream_vacuum"

type VacuumProvisioner struct {
	Config *VacuumConfig
	Logger logging.Logger

	workers    *controller.WorkerController
	streams    *streamcontroller.StreamController
	controller *vacuumcontroller.VacuumController

	definition *worker.Definition
}

// cfg may be nil or sparse.
func NewVacuumProvisioner(ds *iDatastore.PostgresDatastore, cfg *VacuumConfig, logger logging.Logger) (*VacuumProvisioner, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if cfg == nil {
		cfg = &VacuumConfig{}
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

	vacuumController, err := vacuumcontroller.NewVacuumController(ds, logger)
	if err != nil {
		return nil, err
	}

	definition, err := worker.NewDefinition(WorkerStreamVacuum, common.OwnerStream, 0, toVacuumMetadata(cfg))
	if err != nil {
		return nil, err
	}

	return &VacuumProvisioner{
		Config:     cfg,
		Logger:     logger,
		workers:    workers,
		streams:    streams,
		controller: vacuumController,
		definition: definition,
	}, nil
}

func (d *VacuumProvisioner) Definition() *worker.Definition {
	return d.definition
}
