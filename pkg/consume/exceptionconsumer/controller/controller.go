package controller

import (
	"errors"

	"github.com/agentstax/vulkan/pkg/common/logging"
	"github.com/agentstax/vulkan/pkg/consume/exceptionconsumer/controller/datastore"
	iDatastore "github.com/agentstax/vulkan/pkg/datastore"
)

type ExceptionConsumerGroupController struct {
	Logger logging.Logger

	datastore *datastore.ExceptionConsumerGroupDatastore
}

func NewExceptionConsumerGroupController(ds *iDatastore.PostgresDatastore, logger logging.Logger) (*ExceptionConsumerGroupController, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	exceptionConsumerGroupDatastore, err := datastore.NewExceptionConsumerGroupDatastore(ds, logger)
	if err != nil {
		return nil, err
	}

	return &ExceptionConsumerGroupController{
		Logger:    logger,
		datastore: exceptionConsumerGroupDatastore,
	}, nil
}
