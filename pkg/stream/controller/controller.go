package controller

import (
	"errors"
	"fmt"

	"github.com/agentstax/sqlstreams/pkg/common/logging"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	migratecontroller "github.com/agentstax/sqlstreams/pkg/migrate/controller"
	"github.com/agentstax/sqlstreams/pkg/stream/controller/datastore"
	"github.com/agentstax/sqlstreams/pkg/worker"
)

type StreamController struct {
	Logger logging.Logger

	declarers         []worker.Declarer
	datastore         *datastore.StreamDatastore
	migrateController *migratecontroller.Controller
}

// declarers run on every Register to create the registered stream's worker
// rows -- pass them only from a registrar; a controller built for reads
// needs none.
func NewStreamController(ds *iDatastore.PostgresDatastore, logger logging.Logger, declarers ...worker.Declarer) (*StreamController, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}
	for i, declarer := range declarers {
		if declarer == nil {
			return nil, fmt.Errorf("declarer %d must not be nil", i)
		}
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
		declarers:         declarers,
		datastore:         streamDatastore,
		migrateController: migrateController,
	}, nil
}
