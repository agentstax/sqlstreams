package controller

import (
	"errors"
	"fmt"

	"github.com/agentstax/vulkan/pkg/common/logging"
	iDatastore "github.com/agentstax/vulkan/pkg/datastore"
	migratecontroller "github.com/agentstax/vulkan/pkg/migrate/controller"
	"github.com/agentstax/vulkan/pkg/topic/controller/datastore"
	"github.com/agentstax/vulkan/pkg/worker"
)

type TopicController struct {
	Logger logging.Logger

	declarers         []worker.Declarer
	datastore         *datastore.TopicDatastore
	migrateController *migratecontroller.Controller
}

// declarers run on every Register to create the registered topic's worker
// rows -- pass them only from a registrar; a controller built for reads
// needs none.
func NewTopicController(ds *iDatastore.PostgresDatastore, logger logging.Logger, declarers ...worker.Declarer) (*TopicController, error) {
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

	topicDatastore, err := datastore.NewTopicDatastore(ds, logger)
	if err != nil {
		return nil, err
	}

	migrateController, err := migratecontroller.NewController(ds, logger)
	if err != nil {
		return nil, err
	}

	return &TopicController{
		Logger:            logger,
		declarers:         declarers,
		datastore:         topicDatastore,
		migrateController: migrateController,
	}, nil
}
