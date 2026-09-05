package controller

import (
	"errors"

	"github.com/agentstax/vulkan/pkg/common/logging"
	iDatastore "github.com/agentstax/vulkan/pkg/datastore"
	"github.com/agentstax/vulkan/pkg/metrics/controller/datastore"
)

// MetricsController is the single read surface for the DB-snapshot metrics.
type MetricsController struct {
	Logger logging.Logger

	datastore *datastore.MetricsDatastore
}

func NewMetricsController(ds *iDatastore.PostgresDatastore, logger logging.Logger) (*MetricsController, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	metricsDatastore, err := datastore.NewMetricsDatastore(ds, logger)
	if err != nil {
		return nil, err
	}

	return &MetricsController{
		Logger:    logger,
		datastore: metricsDatastore,
	}, nil
}
