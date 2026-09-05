package controller

import (
	"errors"

	"github.com/agentstax/vulkan/pkg/common/logging"
	iDatastore "github.com/agentstax/vulkan/pkg/datastore"
	migratecontroller "github.com/agentstax/vulkan/pkg/migrate/controller"
	"github.com/agentstax/vulkan/pkg/worker/controller/datastore"
)

type WorkerController struct {
	Logger logging.Logger

	datastore         *datastore.WorkerDatastore
	migrateController *migratecontroller.Controller
}

func NewWorkerController(ds *iDatastore.PostgresDatastore, logger logging.Logger) (*WorkerController, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	workerDatastore, err := datastore.NewWorkerDatastore(ds, logger)
	if err != nil {
		return nil, err
	}

	migrateController, err := migratecontroller.NewController(ds, logger)
	if err != nil {
		return nil, err
	}

	return &WorkerController{
		Logger:            logger,
		datastore:         workerDatastore,
		migrateController: migrateController,
	}, nil
}
