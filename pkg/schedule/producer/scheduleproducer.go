package producer

import (
	"errors"

	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/common/logging"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	"github.com/agentstax/sqlstreams/pkg/producer"
	scheduleproducercontroller "github.com/agentstax/sqlstreams/pkg/schedule/producer/controller"
	"github.com/agentstax/sqlstreams/pkg/worker"
	"github.com/agentstax/sqlstreams/pkg/worker/controller"
)

const WorkerScheduleProducer = "schedule_producer"

type ScheduleProducerProvisioner struct {
	Config *ScheduleProducerConfig
	Logger logging.Logger

	ds         *iDatastore.PostgresDatastore
	workers    *controller.WorkerController
	controller *scheduleproducercontroller.ScheduleProducerController
	producer   *producer.Producer // each produce registers an instance on the due row's target stream

	definition *worker.Definition
}

// cfg may be nil or sparse.
func NewScheduleProducerProvisioner(ds *iDatastore.PostgresDatastore, cfg *ScheduleProducerConfig, logger logging.Logger) (*ScheduleProducerProvisioner, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if cfg == nil {
		cfg = &ScheduleProducerConfig{}
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

	schedulerController, err := scheduleproducercontroller.NewScheduleProducerController(ds, logger)
	if err != nil {
		return nil, err
	}

	jobProducer, err := producer.NewProducer(ds)
	if err != nil {
		return nil, err
	}

	definition, err := worker.NewDefinition(WorkerScheduleProducer, common.OwnerSystem, 1, defaultScheduleProducerMetadata())
	if err != nil {
		return nil, err
	}

	return &ScheduleProducerProvisioner{
		Config:     cfg,
		Logger:     logger,
		ds:         ds,
		workers:    workers,
		controller: schedulerController,
		producer:   jobProducer,
		definition: definition,
	}, nil
}

func (d *ScheduleProducerProvisioner) Definition() *worker.Definition {
	return d.definition
}
