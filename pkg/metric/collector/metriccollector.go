package collector

import (
	"errors"

	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/common/logging"
	compactioncontroller "github.com/agentstax/sqlstreams/pkg/compaction/controller"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	metricscontroller "github.com/agentstax/sqlstreams/pkg/metric/controller"
	"github.com/agentstax/sqlstreams/pkg/producer"
	streamcontroller "github.com/agentstax/sqlstreams/pkg/stream/controller"
	"github.com/agentstax/sqlstreams/pkg/worker"
	"github.com/agentstax/sqlstreams/pkg/worker/controller"
)

const WorkerMetricsCollector = "metrics_collector"

type MetricCollectorProvisioner struct {
	Config *MetricCollectorConfig
	Logger logging.Logger

	workers    *controller.WorkerController
	metrics    *metricscontroller.MetricController
	streams    *streamcontroller.StreamController
	alertHeads *compactioncontroller.CompactionController
	producer   *producer.Producer // each Provision registers its own instance from it

	definition *worker.Definition
}

// cfg may be nil or sparse.
func NewMetricsCollectorProvisioner(ds *iDatastore.PostgresDatastore, cfg *MetricCollectorConfig, logger logging.Logger) (*MetricCollectorProvisioner, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if cfg == nil {
		cfg = &MetricCollectorConfig{}
	}
	cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	workers, err := controller.NewWorkerController(ds, logger)
	if err != nil {
		return nil, err
	}

	metricController, err := metricscontroller.NewMetricsController(ds, logger)
	if err != nil {
		return nil, err
	}

	streams, err := streamcontroller.NewStreamController(ds, logger)
	if err != nil {
		return nil, err
	}

	alertHeads, err := compactioncontroller.NewCompactionController(ds, logger)
	if err != nil {
		return nil, err
	}

	measurementProducer, err := producer.NewProducer(ds)
	if err != nil {
		return nil, err
	}

	definition, err := worker.NewDefinition(WorkerMetricsCollector, common.OwnerSystem, 1, toMetricsCollectorMetadata(cfg))
	if err != nil {
		return nil, err
	}

	return &MetricCollectorProvisioner{
		Config:     cfg,
		Logger:     logger,
		workers:    workers,
		metrics:    metricController,
		streams:    streams,
		alertHeads: alertHeads,
		producer:   measurementProducer,
		definition: definition,
	}, nil
}

func (d *MetricCollectorProvisioner) Definition() *worker.Definition {
	return d.definition
}
