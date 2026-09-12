package controller

import (
	"errors"

	"github.com/allegedlyreliable/sqlstreams/pkg/common/logging"
	"github.com/allegedlyreliable/sqlstreams/pkg/consume/controller/datastore"
	iDatastore "github.com/allegedlyreliable/sqlstreams/pkg/datastore"
)

type ConsumeController struct {
	Logger logging.Logger

	datastore *datastore.ConsumeDatastore
}

func NewConsumeController(ds *iDatastore.PostgresDatastore, logger logging.Logger) (*ConsumeController, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	consumerDatastore, err := datastore.NewConsumeDatastore(ds, logger)
	if err != nil {
		return nil, err
	}

	return &ConsumeController{
		Logger:    logger,
		datastore: consumerDatastore,
	}, nil
}
