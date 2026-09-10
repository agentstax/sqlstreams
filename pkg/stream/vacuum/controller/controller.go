package controller

import (
	"errors"

	"github.com/agentstax/sqlstreams/pkg/common/logging"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	"github.com/agentstax/sqlstreams/pkg/stream/vacuum/controller/datastore"
)

type VacuumController struct {
	Logger logging.Logger

	datastore *datastore.VacuumDatastore
}

func NewVacuumController(ds *iDatastore.PostgresDatastore, logger logging.Logger) (*VacuumController, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	vacuumDatastore, err := datastore.NewVacuumDatastore(ds, logger)
	if err != nil {
		return nil, err
	}

	return &VacuumController{
		Logger:    logger,
		datastore: vacuumDatastore,
	}, nil
}
