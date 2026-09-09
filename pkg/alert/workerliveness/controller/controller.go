package controller

import (
	"errors"

	"github.com/agentstax/vulkan/pkg/common/logging"
	iDatastore "github.com/agentstax/vulkan/pkg/datastore"
	metricscontroller "github.com/agentstax/vulkan/pkg/metric/controller"
)

type WorkerLivenessController struct {
	Logger logging.Logger

	metrics *metricscontroller.MetricController
}

func NewWorkerLivenessController(ds *iDatastore.PostgresDatastore, logger logging.Logger) (*WorkerLivenessController, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	metricController, err := metricscontroller.NewMetricsController(ds, logger)
	if err != nil {
		return nil, err
	}

	return &WorkerLivenessController{
		Logger:  logger,
		metrics: metricController,
	}, nil
}
