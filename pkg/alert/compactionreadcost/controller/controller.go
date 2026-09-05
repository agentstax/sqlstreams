package controller

import (
	"errors"

	"github.com/agentstax/vulkan/pkg/alert/compactionreadcost/controller/datastore"
	"github.com/agentstax/vulkan/pkg/common/logging"
	iDatastore "github.com/agentstax/vulkan/pkg/datastore"
)

type CompactionReadCostController struct {
	Logger logging.Logger

	datastore *datastore.CompactionReadCostDatastore
}

func NewCompactionReadCostController(ds *iDatastore.PostgresDatastore, logger logging.Logger) (*CompactionReadCostController, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	compactionReadCostDatastore, err := datastore.NewCompactionReadCostDatastore(ds, logger)
	if err != nil {
		return nil, err
	}

	return &CompactionReadCostController{
		Logger:    logger,
		datastore: compactionReadCostDatastore,
	}, nil
}
