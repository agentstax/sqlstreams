package partitioncount

import (
	"errors"

	"github.com/allegedlyreliable/sqlstreams/pkg/alert/partitioncount/controller"
	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/common/logging"
	compactioncontroller "github.com/allegedlyreliable/sqlstreams/pkg/compaction/controller"
	consumecontroller "github.com/allegedlyreliable/sqlstreams/pkg/consume/controller"
	"github.com/allegedlyreliable/sqlstreams/pkg/consumer"
	iDatastore "github.com/allegedlyreliable/sqlstreams/pkg/datastore"
	"github.com/allegedlyreliable/sqlstreams/pkg/producer"
	streamcontroller "github.com/allegedlyreliable/sqlstreams/pkg/stream/controller"
	"github.com/allegedlyreliable/sqlstreams/pkg/worker"
	workercontroller "github.com/allegedlyreliable/sqlstreams/pkg/worker/controller"
)

// PartitionCountProvisioner is the alert's worker kind: one row owning the
// alert's consumer group on the schedules stream.
type PartitionCountProvisioner struct {
	Config *PartitionCountConfig
	Logger logging.Logger

	ds               *iDatastore.PostgresDatastore
	workers          *workercontroller.WorkerController
	streams          *streamcontroller.StreamController
	consumers        *consumecontroller.ConsumeController
	controller       *controller.PartitionCountController
	producer         *producer.Producer
	alertHeads       *compactioncontroller.CompactionController
	scheduleConsumer *consumer.Consumer

	definition *worker.Definition
}

// cfg may be nil or sparse.
func NewPartitionCountProvisioner(ds *iDatastore.PostgresDatastore, cfg *PartitionCountConfig, logger logging.Logger) (*PartitionCountProvisioner, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if cfg == nil {
		cfg = &PartitionCountConfig{}
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

	streams, err := streamcontroller.NewStreamController(ds, logger)
	if err != nil {
		return nil, err
	}

	consumers, err := consumecontroller.NewConsumeController(ds, logger)
	if err != nil {
		return nil, err
	}

	partitionCountController, err := controller.NewPartitionCountController(ds, logger)
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

	definition, err := worker.NewDefinition(JobName, common.OwnerConsumerGroup, 1, toPartitionCountMetadata(cfg))
	if err != nil {
		return nil, err
	}

	return &PartitionCountProvisioner{
		Config:           cfg,
		Logger:           logger,
		ds:               ds,
		workers:          workers,
		streams:          streams,
		consumers:        consumers,
		controller:       partitionCountController,
		producer:         alertProducer,
		alertHeads:       alertHeads,
		scheduleConsumer: scheduleConsumer,
		definition:       definition,
	}, nil
}

func (d *PartitionCountProvisioner) Definition() *worker.Definition {
	return d.definition
}
