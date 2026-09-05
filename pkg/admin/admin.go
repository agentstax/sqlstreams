package admin

import (
	"github.com/agentstax/vulkan/pkg/alert/compactionreadcost"
	"github.com/agentstax/vulkan/pkg/alert/partitioncount"
	"github.com/agentstax/vulkan/pkg/alert/workerliveness"
	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/common/logging"
	compactioncontroller "github.com/agentstax/vulkan/pkg/compaction/controller"
	consumecontroller "github.com/agentstax/vulkan/pkg/consume/controller"
	consumejanitor "github.com/agentstax/vulkan/pkg/consume/janitor"
	"github.com/agentstax/vulkan/pkg/datastore"
	"github.com/agentstax/vulkan/pkg/metrics/collector"
	metricscontroller "github.com/agentstax/vulkan/pkg/metrics/controller"
	migratecontroller "github.com/agentstax/vulkan/pkg/migrate/controller"
	"github.com/agentstax/vulkan/pkg/producer"
	schedulecontroller "github.com/agentstax/vulkan/pkg/schedule/controller"
	scheduleproducer "github.com/agentstax/vulkan/pkg/schedule/producer"
	"github.com/agentstax/vulkan/pkg/scheduler"
	systemcontroller "github.com/agentstax/vulkan/pkg/system/controller"
	topiccontroller "github.com/agentstax/vulkan/pkg/topic/controller"
	topicjanitor "github.com/agentstax/vulkan/pkg/topic/janitor"
	"github.com/agentstax/vulkan/pkg/worker"
	workercontroller "github.com/agentstax/vulkan/pkg/worker/controller"
	"github.com/agentstax/vulkan/pkg/worker/manager"
)

type MessageAdmin struct {
	Logger logging.Logger
	Retry  *common.RetryPolicy

	systemController   *systemcontroller.SystemController
	topicController    *topiccontroller.TopicController
	scheduleController *schedulecontroller.ScheduleController
	consumerController *consumecontroller.ConsumeController
	scheduleProducer   *producer.Producer
	scheduler          *scheduler.Scheduler
	heads              *compactioncontroller.CompactionController
	metricsController  *metricscontroller.MetricsController
	workerController   *workercontroller.WorkerController
	migrateController  *migratecontroller.Controller
	alertDeclarers     []worker.Declarer
	allowDestroy       bool
}

func NewMessageAdmin(ds *datastore.PostgresDatastore, cfg *MessageAdminConfig) (*MessageAdmin, error) {
	if cfg == nil {
		cfg = &MessageAdminConfig{}
	}
	cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	scheduleProducerProvisioner, err := scheduleproducer.NewScheduleProducerProvisioner(ds, nil, ds.Logger)
	if err != nil {
		return nil, err
	}

	topicJanitorProvisioner, err := topicjanitor.NewJanitorProvisioner(ds, nil, ds.Logger)
	if err != nil {
		return nil, err
	}

	consumerGroupJanitorProvisioner, err := consumejanitor.NewJanitorProvisioner(ds, nil, ds.Logger)
	if err != nil {
		return nil, err
	}

	metricsCollectorProvisioner, err := collector.NewMetricsCollectorProvisioner(ds, nil, ds.Logger)
	if err != nil {
		return nil, err
	}

	// a declarer here, never run -- admin creates manager rows, it doesn't claim them
	managerProvisioner, err := manager.NewManagerProvisioner(ds, 1, nil, ds.Logger, topicJanitorProvisioner, scheduleProducerProvisioner, metricsCollectorProvisioner)
	if err != nil {
		return nil, err
	}

	systemController, err := systemcontroller.NewSystemController(ds, ds.Logger, scheduleProducerProvisioner, metricsCollectorProvisioner, consumerGroupJanitorProvisioner, managerProvisioner)
	if err != nil {
		return nil, err
	}

	topicController, err := topiccontroller.NewTopicController(ds, ds.Logger, topicJanitorProvisioner)
	if err != nil {
		return nil, err
	}

	scheduleController, err := schedulecontroller.NewScheduleController(ds, ds.Logger)
	if err != nil {
		return nil, err
	}

	scheduleProducer, err := producer.NewProducer(ds)
	if err != nil {
		return nil, err
	}

	heads, err := compactioncontroller.NewCompactionController(ds, ds.Logger)
	if err != nil {
		return nil, err
	}

	consumerController, err := consumecontroller.NewConsumeController(ds, ds.Logger)
	if err != nil {
		return nil, err
	}

	metricsController, err := metricscontroller.NewMetricsController(ds, ds.Logger)
	if err != nil {
		return nil, err
	}

	workerController, err := workercontroller.NewWorkerController(ds, ds.Logger)
	if err != nil {
		return nil, err
	}

	// declarers here, never run -- RegisterSystem creates the alerts' consumer
	// groups and worker rows, the system manager claims them
	partitionCountProvisioner, err := partitioncount.NewPartitionCountProvisioner(ds, nil, ds.Logger)
	if err != nil {
		return nil, err
	}
	compactionReadCostProvisioner, err := compactionreadcost.NewCompactionReadCostProvisioner(ds, nil, ds.Logger)
	if err != nil {
		return nil, err
	}

	workerLivenessProvisioner, err := workerliveness.NewWorkerLivenessProvisioner(ds, nil, ds.Logger)
	if err != nil {
		return nil, err
	}

	migrateController, err := migratecontroller.NewController(ds, ds.Logger)
	if err != nil {
		return nil, err
	}

	alertScheduler, err := scheduler.NewScheduler(ds)
	if err != nil {
		return nil, err
	}

	return &MessageAdmin{
		Logger:             ds.Logger,
		Retry:              ds.Retry,
		systemController:   systemController,
		topicController:    topicController,
		scheduleController: scheduleController,
		scheduler:          alertScheduler,
		consumerController: consumerController,
		scheduleProducer:   scheduleProducer,
		heads:              heads,
		metricsController:  metricsController,
		workerController:   workerController,
		migrateController:  migrateController,
		alertDeclarers:     []worker.Declarer{partitionCountProvisioner, compactionReadCostProvisioner, workerLivenessProvisioner},
		allowDestroy:       cfg.AllowDestroy,
	}, nil
}
