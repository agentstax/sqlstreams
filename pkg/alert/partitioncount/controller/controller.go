package controller

import (
	"errors"

	"github.com/agentstax/vulkan/pkg/alert/partitioncount/controller/datastore"
	"github.com/agentstax/vulkan/pkg/common/logging"
	iDatastore "github.com/agentstax/vulkan/pkg/datastore"
	metricscontroller "github.com/agentstax/vulkan/pkg/metric/controller"
)

type PartitionCountController struct {
	Logger logging.Logger

	datastore *datastore.PartitionCountDatastore
	metrics   *metricscontroller.MetricController
}

func NewPartitionCountController(ds *iDatastore.PostgresDatastore, logger logging.Logger) (*PartitionCountController, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	partitionCountDatastore, err := datastore.NewPartitionCountDatastore(ds, logger)
	if err != nil {
		return nil, err
	}
	metricController, err := metricscontroller.NewMetricsController(ds, logger)
	if err != nil {
		return nil, err
	}

	return &PartitionCountController{
		Logger:    logger,
		datastore: partitionCountDatastore,
		metrics:   metricController,
	}, nil
}
