package controller

import (
	"errors"

	"github.com/agentstax/sqlstreams/pkg/common/logging"
	"github.com/agentstax/sqlstreams/pkg/datastore"
	metricscontroller "github.com/agentstax/sqlstreams/pkg/metric/controller"
	workercontroller "github.com/agentstax/sqlstreams/pkg/worker/controller"
)

type CollectorProgressController struct {
	Logger logging.Logger

	metrics *metricscontroller.MetricController
	workers *workercontroller.WorkerController
}

func NewCollectorProgressController(ds *datastore.PostgresDatastore, logger logging.Logger) (*CollectorProgressController, error) {
	if ds == nil {
		return nil, errors.New("ds must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}
	measurements, err := metricscontroller.NewMetricsController(ds, logger)
	if err != nil {
		return nil, err
	}
	workers, err := workercontroller.NewWorkerController(ds, logger)
	if err != nil {
		return nil, err
	}
	return &CollectorProgressController{Logger: logger, metrics: measurements, workers: workers}, nil
}
