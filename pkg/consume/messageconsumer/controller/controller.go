package controller

import (
	"errors"

	"github.com/agentstax/vulkan/pkg/common/logging"
	"github.com/agentstax/vulkan/pkg/consume/messageconsumer/controller/datastore"
	iDatastore "github.com/agentstax/vulkan/pkg/datastore"
)

type MessageConsumerGroupController struct {
	Logger logging.Logger

	datastore *datastore.MessageConsumerGroupDatastore
}

func NewMessageConsumerGroupController(ds *iDatastore.PostgresDatastore, logger logging.Logger) (*MessageConsumerGroupController, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	messageConsumerGroupDatastore, err := datastore.NewMessageConsumerGroupDatastore(ds, logger)
	if err != nil {
		return nil, err
	}

	return &MessageConsumerGroupController{
		Logger:    logger,
		datastore: messageConsumerGroupDatastore,
	}, nil
}
