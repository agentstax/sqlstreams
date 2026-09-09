package controller

import (
	"errors"

	"github.com/agentstax/sqlstreams/pkg/common/logging"
	"github.com/agentstax/sqlstreams/pkg/consume/base/controller/datastore"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
)

type KeyLeaseController struct {
	Logger logging.Logger

	datastore *datastore.KeyLeaseDatastore
}

func NewKeyLeaseController(ds *iDatastore.PostgresDatastore, logger logging.Logger) (*KeyLeaseController, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	keyLeaseDatastore, err := datastore.NewKeyLeaseDatastore(ds, logger)
	if err != nil {
		return nil, err
	}

	return &KeyLeaseController{
		Logger:    logger,
		datastore: keyLeaseDatastore,
	}, nil
}
