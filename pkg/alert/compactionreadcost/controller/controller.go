package controller

import (
	"errors"

	"github.com/agentstax/vulkan/pkg/common/logging"
	iDatastore "github.com/agentstax/vulkan/pkg/datastore"
	metricscontroller "github.com/agentstax/vulkan/pkg/metrics/controller"
)

type CompactionReadCostController struct {
	Logger logging.Logger

	metrics *metricscontroller.MetricsController
}

func NewCompactionReadCostController(ds *iDatastore.PostgresDatastore, logger logging.Logger) (*CompactionReadCostController, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	metrics, err := metricscontroller.NewMetricsController(ds, logger)
	if err != nil {
		return nil, err
	}

	return &CompactionReadCostController{
		Logger:  logger,
		metrics: metrics,
	}, nil
}
