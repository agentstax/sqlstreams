package compactionreadcost

import (
	"errors"

	"github.com/agentstax/vulkan/pkg/alert/compactionreadcost/controller"
	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/common/logging"
	compactioncontroller "github.com/agentstax/vulkan/pkg/compaction/controller"
	consumecontroller "github.com/agentstax/vulkan/pkg/consume/controller"
	"github.com/agentstax/vulkan/pkg/consumer"
	iDatastore "github.com/agentstax/vulkan/pkg/datastore"
	"github.com/agentstax/vulkan/pkg/producer"
	topiccontroller "github.com/agentstax/vulkan/pkg/topic/controller"
	"github.com/agentstax/vulkan/pkg/worker"
	workercontroller "github.com/agentstax/vulkan/pkg/worker/controller"
)

// CompactionReadCostProvisioner is the alert's worker kind: one row owning the
// alert's consumer group on the schedules topic.
type CompactionReadCostProvisioner struct {
	Config *CompactionReadCostConfig
	Logger logging.Logger

	ds               *iDatastore.PostgresDatastore
	workers          *workercontroller.WorkerController
	topics           *topiccontroller.TopicController
	consumers        *consumecontroller.ConsumeController
	controller       *controller.CompactionReadCostController
	producer         *producer.Producer
	alertHeads       *compactioncontroller.CompactionController
	scheduleConsumer *consumer.Consumer

	definition *worker.Definition
}

// cfg may be nil or sparse.
func NewCompactionReadCostProvisioner(ds *iDatastore.PostgresDatastore, cfg *CompactionReadCostConfig, logger logging.Logger) (*CompactionReadCostProvisioner, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if cfg == nil {
		cfg = &CompactionReadCostConfig{}
	}
	cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	workers, err := workercontroller.NewWorkerController(ds, logger)
	if err != nil {
		return nil, err
	}

	topics, err := topiccontroller.NewTopicController(ds, logger)
	if err != nil {
		return nil, err
	}

	consumers, err := consumecontroller.NewConsumeController(ds, logger)
	if err != nil {
		return nil, err
	}

	compactionReadCostController, err := controller.NewCompactionReadCostController(ds, logger)
	if err != nil {
		return nil, err
	}

	alertProducer, err := producer.NewProducer(ds)
	if err != nil {
		return nil, err
	}

	alertHeads, err := compactioncontroller.NewCompactionController(ds, logger)
	if err != nil {
		return nil, err
	}

	scheduleConsumer, err := consumer.NewConsumer(ds)
	if err != nil {
		return nil, err
	}

	definition, err := worker.NewDefinition(JobName, common.OwnerConsumerGroup, 1, toCompactionReadCostMetadata(cfg))
	if err != nil {
		return nil, err
	}

	return &CompactionReadCostProvisioner{
		Config:           cfg,
		Logger:           logger,
		ds:               ds,
		workers:          workers,
		topics:           topics,
		consumers:        consumers,
		controller:       compactionReadCostController,
		producer:         alertProducer,
		alertHeads:       alertHeads,
		scheduleConsumer: scheduleConsumer,
		definition:       definition,
	}, nil
}

func (d *CompactionReadCostProvisioner) Definition() *worker.Definition {
	return d.definition
}
