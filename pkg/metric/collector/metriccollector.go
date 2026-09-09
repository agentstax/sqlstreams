package collector

import (
	"errors"

	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/common/logging"
	compactioncontroller "github.com/agentstax/vulkan/pkg/compaction/controller"
	iDatastore "github.com/agentstax/vulkan/pkg/datastore"
	metricscontroller "github.com/agentstax/vulkan/pkg/metric/controller"
	"github.com/agentstax/vulkan/pkg/producer"
	topiccontroller "github.com/agentstax/vulkan/pkg/topic/controller"
	"github.com/agentstax/vulkan/pkg/worker"
	"github.com/agentstax/vulkan/pkg/worker/controller"
)

const WorkerMetricsCollector = "metrics_collector"

type MetricCollectorProvisioner struct {
	Config *MetricCollectorConfig
	Logger logging.Logger

	workers    *controller.WorkerController
	metrics    *metricscontroller.MetricController
	topics     *topiccontroller.TopicController
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

	topics, err := topiccontroller.NewTopicController(ds, logger)
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
		topics:     topics,
		alertHeads: alertHeads,
		producer:   measurementProducer,
		definition: definition,
	}, nil
}

func (d *MetricCollectorProvisioner) Definition() *worker.Definition {
	return d.definition
}
