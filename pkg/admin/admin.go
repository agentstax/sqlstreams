package admin

import (
	"github.com/agentstax/sqlstreams/pkg/alert"
	"github.com/agentstax/sqlstreams/pkg/alert/collectorprogress"
	collectorprogresscontroller "github.com/agentstax/sqlstreams/pkg/alert/collectorprogress/controller"
	"github.com/agentstax/sqlstreams/pkg/alert/compactionreadcost"
	compactionreadcostcontroller "github.com/agentstax/sqlstreams/pkg/alert/compactionreadcost/controller"
	"github.com/agentstax/sqlstreams/pkg/alert/partitioncount"
	partitioncountcontroller "github.com/agentstax/sqlstreams/pkg/alert/partitioncount/controller"
	"github.com/agentstax/sqlstreams/pkg/alert/workerliveness"
	workerlivenesscontroller "github.com/agentstax/sqlstreams/pkg/alert/workerliveness/controller"
	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/common/logging"
	compactioncontroller "github.com/agentstax/sqlstreams/pkg/compaction/controller"
	consumecontroller "github.com/agentstax/sqlstreams/pkg/consume/controller"
	consumejanitor "github.com/agentstax/sqlstreams/pkg/consume/janitor"
	"github.com/agentstax/sqlstreams/pkg/datastore"
	"github.com/agentstax/sqlstreams/pkg/metric/collector"
	metricscontroller "github.com/agentstax/sqlstreams/pkg/metric/controller"
	migratecontroller "github.com/agentstax/sqlstreams/pkg/migrate/controller"
	schedulecontroller "github.com/agentstax/sqlstreams/pkg/schedule/controller"
	scheduleproducer "github.com/agentstax/sqlstreams/pkg/schedule/producer"
	"github.com/agentstax/sqlstreams/pkg/scheduler"
	streamcontroller "github.com/agentstax/sqlstreams/pkg/stream/controller"
	streamjanitor "github.com/agentstax/sqlstreams/pkg/stream/janitor"
	"github.com/agentstax/sqlstreams/pkg/stream/vacuum"
	systemcontroller "github.com/agentstax/sqlstreams/pkg/system/controller"
	"github.com/agentstax/sqlstreams/pkg/worker"
	workercontroller "github.com/agentstax/sqlstreams/pkg/worker/controller"
	"github.com/agentstax/sqlstreams/pkg/worker/manager"
)

type MessageAdmin struct {
	Logger logging.Logger
	Retry  *common.RetryPolicy

	ds                 *datastore.PostgresDatastore
	systemController   *systemcontroller.SystemController
	streamController   *streamcontroller.StreamController
	scheduleController *schedulecontroller.ScheduleController
	consumerController *consumecontroller.ConsumeController
	scheduler          *scheduler.Scheduler
	heads              *compactioncontroller.CompactionController
	metricController   *metricscontroller.MetricController
	workerController   *workercontroller.WorkerController
	migrateController  *migratecontroller.Controller
	systemDeclarers    []worker.Declarer
	alertDeclarers     []worker.Declarer
	alertEvaluators    map[string]alert.Evaluator
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

	streamJanitorProvisioner, err := streamjanitor.NewJanitorProvisioner(ds, nil, ds.Logger)
	if err != nil {
		return nil, err
	}

	streamVacuumProvisioner, err := vacuum.NewVacuumProvisioner(ds, nil, ds.Logger)
	if err != nil {
		return nil, err
	}

	consumerGroupJanitorProvisioner, err := consumejanitor.NewJanitorProvisioner(ds, nil, ds.Logger)
	if err != nil {
		return nil, err
	}

	metricCollectorProvisioner, err := collector.NewMetricsCollectorProvisioner(ds, nil, ds.Logger)
	if err != nil {
		return nil, err
	}

	// a declarer here, never run -- admin creates manager rows, it doesn't claim them
	managerProvisioner, err := manager.NewManagerProvisioner(ds, 1, nil, ds.Logger, streamJanitorProvisioner, streamVacuumProvisioner, scheduleProducerProvisioner, metricCollectorProvisioner)
	if err != nil {
		return nil, err
	}

	systemController, err := systemcontroller.NewSystemController(ds, ds.Logger)
	if err != nil {
		return nil, err
	}

	streamController, err := streamcontroller.NewStreamController(ds, ds.Logger)
	if err != nil {
		return nil, err
	}

	scheduleController, err := schedulecontroller.NewScheduleController(ds, ds.Logger)
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

	metricController, err := metricscontroller.NewMetricsController(ds, ds.Logger)
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
	collectorProgressProvisioner, err := collectorprogress.NewCollectorProgressProvisioner(ds, nil, ds.Logger)
	if err != nil {
		return nil, err
	}

	migrateController, err := migratecontroller.NewController(ds, ds.Logger)
	if err != nil {
		return nil, err
	}

	partitionCountController, err := partitioncountcontroller.NewPartitionCountController(ds, ds.Logger)
	if err != nil {
		return nil, err
	}
	compactionReadCostController, err := compactionreadcostcontroller.NewCompactionReadCostController(ds, ds.Logger)
	if err != nil {
		return nil, err
	}
	workerLivenessController, err := workerlivenesscontroller.NewWorkerLivenessController(ds, ds.Logger)
	if err != nil {
		return nil, err
	}
	collectorProgressController, err := collectorprogresscontroller.NewCollectorProgressController(ds, ds.Logger)
	if err != nil {
		return nil, err
	}

	alertScheduler, err := scheduler.NewScheduler(ds)
	if err != nil {
		return nil, err
	}

	return &MessageAdmin{
		ds:                 ds,
		Logger:             ds.Logger,
		Retry:              ds.Retry,
		systemController:   systemController,
		streamController:   streamController,
		scheduleController: scheduleController,
		scheduler:          alertScheduler,
		consumerController: consumerController,
		heads:              heads,
		metricController:   metricController,
		workerController:   workerController,
		migrateController:  migrateController,
		systemDeclarers:    []worker.Declarer{scheduleProducerProvisioner, consumerGroupJanitorProvisioner, managerProvisioner},
		alertDeclarers:     []worker.Declarer{partitionCountProvisioner, compactionReadCostProvisioner, workerLivenessProvisioner, collectorProgressProvisioner},
		alertEvaluators: map[string]alert.Evaluator{
			alert.AlertPartitionCount.Name:           partitionCountController,
			alert.AlertCompactionReadCost.Name:       compactionReadCostController,
			alert.AlertWorkerLiveness.Name:           workerLivenessController,
			alert.AlertMetricsCollectorProgress.Name: collectorProgressController,
		},
		allowDestroy: cfg.AllowDestroy,
	}, nil
}
