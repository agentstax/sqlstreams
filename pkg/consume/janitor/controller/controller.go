package controller

import (
	"errors"

	"github.com/agentstax/vulkan/pkg/common/logging"
	"github.com/agentstax/vulkan/pkg/consume/janitor/controller/datastore"
	iDatastore "github.com/agentstax/vulkan/pkg/datastore"
)

// JanitorController is the consumer group janitor kind's only path to
// persistence: the instance sweeps waiting binding_log rows through it.
type JanitorController struct {
	Logger logging.Logger

	datastore *datastore.JanitorDatastore
}

func NewJanitorController(ds *iDatastore.PostgresDatastore, logger logging.Logger) (*JanitorController, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	janitorDatastore, err := datastore.NewJanitorDatastore(ds, logger)
	if err != nil {
		return nil, err
	}

	return &JanitorController{
		Logger:    logger,
		datastore: janitorDatastore,
	}, nil
}
