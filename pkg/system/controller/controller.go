package controller

import (
	"errors"

	"github.com/agentstax/sqlstreams/pkg/common/logging"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	"github.com/agentstax/sqlstreams/pkg/system/controller/datastore"
)

type SystemController struct {
	Logger logging.Logger

	datastore *datastore.SystemDatastore
}

func NewSystemController(ds *iDatastore.PostgresDatastore, logger logging.Logger) (*SystemController, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	systemDatastore, err := datastore.NewSystemDatastore(ds, logger)
	if err != nil {
		return nil, err
	}

	return &SystemController{
		Logger:    logger,
		datastore: systemDatastore,
	}, nil
}
