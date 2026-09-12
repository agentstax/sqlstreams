package controller

import (
	"errors"

	"github.com/allegedlyreliable/sqlstreams/pkg/alert/partitioncount/controller/datastore"
	"github.com/allegedlyreliable/sqlstreams/pkg/common/logging"
	iDatastore "github.com/allegedlyreliable/sqlstreams/pkg/datastore"
	metricscontroller "github.com/allegedlyreliable/sqlstreams/pkg/metric/controller"
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
