package controller

import (
	"errors"

	"github.com/agentstax/sqlstreams/pkg/common/logging"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	migratecontroller "github.com/agentstax/sqlstreams/pkg/migrate/controller"
	"github.com/agentstax/sqlstreams/pkg/stream/controller/datastore"
)

type StreamController struct {
	Logger logging.Logger

	datastore         *datastore.StreamDatastore
	migrateController *migratecontroller.Controller
}

func NewStreamController(ds *iDatastore.PostgresDatastore, logger logging.Logger) (*StreamController, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	streamDatastore, err := datastore.NewStreamDatastore(ds, logger)
	if err != nil {
		return nil, err
	}

	migrateController, err := migratecontroller.NewController(ds, logger)
	if err != nil {
		return nil, err
	}

	return &StreamController{
		Logger:            logger,
		datastore:         streamDatastore,
		migrateController: migrateController,
	}, nil
}
