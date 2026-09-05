package controller

import (
	"errors"

	"github.com/agentstax/vulkan/pkg/common/logging"
	compactioncontroller "github.com/agentstax/vulkan/pkg/compaction/controller"
	iDatastore "github.com/agentstax/vulkan/pkg/datastore"
	"github.com/agentstax/vulkan/pkg/produce/controller/datastore"
)

type ProduceController struct {
	Logger logging.Logger

	datastore *datastore.ProduceDatastore
	heads     *compactioncontroller.CompactionController
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
	compactionController, err := compactioncontroller.NewCompactionController(ds, logger)
	if err != nil {
		return nil, err
	}

	return &ProduceController{
		Logger:    logger,
		datastore: produceDatastore,
		heads:     compactionController,
	}, nil
}
