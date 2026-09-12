package controller

import (
	"errors"

	"github.com/allegedlyreliable/sqlstreams/pkg/common/logging"
	iDatastore "github.com/allegedlyreliable/sqlstreams/pkg/datastore"
	"github.com/allegedlyreliable/sqlstreams/pkg/produce/controller/datastore"
)

type ProduceController struct {
	Logger logging.Logger

	datastore *datastore.ProduceDatastore
}

func NewProduceController(ds *iDatastore.PostgresDatastore, logger logging.Logger) (*ProduceController, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	produceDatastore, err := datastore.NewProduceDatastore(ds, logger)
	if err != nil {
		return nil, err
	}
	return &ProduceController{
		Logger:    logger,
		datastore: produceDatastore,
	}, nil
}
