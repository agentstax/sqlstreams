package controller

import (
	"errors"

	"github.com/agentstax/sqlstreams/pkg/common/logging"
	"github.com/agentstax/sqlstreams/pkg/consume/deliveryconsumer/controller/datastore"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
)

type DeliveryConsumerGroupController struct {
	Logger logging.Logger

	datastore *datastore.DeliveryConsumerGroupDatastore
}

func NewDeliveryConsumerGroupController(ds *iDatastore.PostgresDatastore, logger logging.Logger) (*DeliveryConsumerGroupController, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	deliveryConsumerGroupDatastore, err := datastore.NewDeliveryConsumerGroupDatastore(ds, logger)
	if err != nil {
		return nil, err
	}

	return &DeliveryConsumerGroupController{
		Logger:    logger,
		datastore: deliveryConsumerGroupDatastore,
	}, nil
}
