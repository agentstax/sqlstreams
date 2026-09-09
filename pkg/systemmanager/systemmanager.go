package systemmanager

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/agentstax/sqlstreams/pkg/alert/collectorprogress"
	"github.com/agentstax/sqlstreams/pkg/alert/compactionreadcost"
	"github.com/agentstax/sqlstreams/pkg/alert/partitioncount"
	"github.com/agentstax/sqlstreams/pkg/alert/workerliveness"
	"github.com/agentstax/sqlstreams/pkg/common/logging"
	"github.com/agentstax/sqlstreams/pkg/consume/cursoradvancer"
	consumejanitor "github.com/agentstax/sqlstreams/pkg/consume/janitor"
	"github.com/agentstax/sqlstreams/pkg/datastore"
	"github.com/agentstax/sqlstreams/pkg/metric/collector"
	migratecontroller "github.com/agentstax/sqlstreams/pkg/migrate/controller"
	scheduleproducer "github.com/agentstax/sqlstreams/pkg/schedule/producer"
	streamjanitor "github.com/agentstax/sqlstreams/pkg/stream/janitor"
	"github.com/agentstax/sqlstreams/pkg/system"
	"github.com/agentstax/sqlstreams/pkg/worker"
	"github.com/agentstax/sqlstreams/pkg/worker/manager"
)

// SystemManager keeps the deployment's upkeep running with no user process
// up: it claims the system's manager row and reconciles every worker row in
// the deployment, the alerts' consumers included. Safe to run N-way -- the
// manager row's own claim admits one reconcile loop at a time, and the
// spawned workers' claims arbitrate the rest.
type SystemManager struct {
	Config *SystemManagerConfig
	Logger logging.Logger

	ds                *datastore.PostgresDatastore
	manager           *manager.ManagerProvisioner
	migrateController *migratecontroller.Controller
}

// cfg may be nil or sparse.
func NewSystemManager(ds *datastore.PostgresDatastore, cfg *SystemManagerConfig) (*SystemManager, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if cfg == nil {
		cfg = &SystemManagerConfig{}
	}
	cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	logger := logging.NewPipelineLogger(ds.Logger, &logging.PipelineLoggerConfig{Buffer: true, Suppress: true})

	streamJanitorProvisioner, err := streamjanitor.NewJanitorProvisioner(ds, nil, logger)
	if err != nil {
		return nil, err
	}

	consumerGroupJanitorProvisioner, err := consumejanitor.NewJanitorProvisioner(ds, nil, logger)
	if err != nil {
		return nil, err
	}

	scheduleProducerProvisioner, err := scheduleproducer.NewScheduleProducerProvisioner(ds, nil, logger)
	if err != nil {
		return nil, err
	}

	// committed keeps advancing -- and retention keeps moving -- for groups
	// whose consumers are offline
	cursorAdvancerProvisioner, err := cursoradvancer.NewCursorAdvancerProvisioner(ds, nil, logger)
	if err != nil {
		return nil, err
	}

	metricCollectorProvisioner, err := collector.NewMetricsCollectorProvisioner(ds, nil, logger)
	if err != nil {
		return nil, err
	}

	partitionCountProvisioner, err := partitioncount.NewPartitionCountProvisioner(ds, nil, logger)
	if err != nil {
		return nil, err
	}
	compactionReadCostProvisioner, err := compactionreadcost.NewCompactionReadCostProvisioner(ds, nil, logger)
	if err != nil {
		return nil, err
	}

	workerLivenessProvisioner, err := workerliveness.NewWorkerLivenessProvisioner(ds, nil, logger)
	if err != nil {
		return nil, err
	}
	collectorProgressProvisioner, err := collectorprogress.NewCollectorProgressProvisioner(ds, nil, logger)
	if err != nil {
		return nil, err
	}

	provisioners := []worker.Provisioner{
		streamJanitorProvisioner,
		consumerGroupJanitorProvisioner,
		scheduleProducerProvisioner,
		metricCollectorProvisioner,
		cursorAdvancerProvisioner,
		partitionCountProvisioner,
		compactionReadCostProvisioner,
		workerLivenessProvisioner,
		collectorProgressProvisioner,
	}
	managerProvisioner, err := manager.NewManagerProvisioner(ds, 1, nil, logger, provisioners...)
	if err != nil {
		return nil, err
	}

	migrateController, err := migratecontroller.NewController(ds, logger)
	if err != nil {
		return nil, err
	}

	return &SystemManager{
		Config:            cfg,
		Logger:            logger,
		ds:                ds,
		manager:           managerProvisioner,
		migrateController: migrateController,
	}, nil
}

// Run keeps the deployment's upkeep running until ctx cancels, and returns
// nil then. Safe to call N times, in one process or many -- the manager
// row's claim admits one reconcile loop at a time and every other call
// retries the claim. A life that ends on its own is logged and claimed
// again behind Config.RunRetry; errors before the first claim return to
// the caller.
// Returns migrate.ErrNotRegistered when no system has been registered.
func (s *SystemManager) Run(ctx context.Context) error {
	owner, err := s.migrateController.SystemOwner(ctx)
	if err != nil {
		// a cancel during the owner read is a requested stop, not a failure
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	runner, err := manager.NewRunner(s.manager, owner, nil, s.Logger)
	if err != nil {
		return err
	}

	for attempt := 0; ; attempt++ {
		err := runner.Run(ctx)
		if ctx.Err() != nil {
			return nil
		}

		// re-jittered every retry -- replicas that hit the same fault must
		// not retry in step
		jitter := 1 + s.Config.JitterFraction*(2*rand.Float64()-1)
		delay := time.Duration(float64(s.Config.RunRetry.CalculateDelay(min(attempt, s.Config.RunRetry.MaxRetries))) * jitter)
		if err != nil {
			s.Logger.ErrorContext(ctx, system.EventSystemManagerStopped.Message(),
				"code", system.EventSystemManagerStopped.GetCode(),
				"attempt", attempt+1,
				"delay", delay,
				"error", err)
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}
