package admin

import (
	"context"
	"strings"

	"github.com/agentstax/sqlstreams/pkg/alert"
	"github.com/agentstax/sqlstreams/pkg/alert/collectorprogress"
	"github.com/agentstax/sqlstreams/pkg/alert/compactionreadcost"
	alertcontroller "github.com/agentstax/sqlstreams/pkg/alert/controller"
	"github.com/agentstax/sqlstreams/pkg/alert/partitioncount"
	"github.com/agentstax/sqlstreams/pkg/alert/workerliveness"
	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/metric"
	"github.com/agentstax/sqlstreams/pkg/metric/collector"
	metricscontroller "github.com/agentstax/sqlstreams/pkg/metric/controller"
	"github.com/agentstax/sqlstreams/pkg/migrate"
	"github.com/agentstax/sqlstreams/pkg/schedule"
	schedulecontroller "github.com/agentstax/sqlstreams/pkg/schedule/controller"
	"github.com/agentstax/sqlstreams/pkg/scheduler"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"github.com/agentstax/sqlstreams/pkg/system"
	systemMigrations "github.com/agentstax/sqlstreams/pkg/system/migrations"
)

// RegisterSystem stands up the shared control-plane tables every stream uses.
// The first RegisterStream against an empty database runs it with a nil cfg,
// so calling it directly matters when cfg does. Safe to call on every startup:
// cfg is applied on every call, so changing a value and redeploying changes
// the system's streams, its built-in alerts' schedules, and its collector rate.
//   - cfg: may be nil or sparse
func (a *MessageAdmin) RegisterSystem(ctx context.Context, cfg *system.SystemConfig) error {
	if cfg == nil {
		cfg = &system.SystemConfig{}
	}
	cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return err
	}

	// the built-in schedules are parsed before the first write
	partitionCountJob, err := partitioncount.NewJob(cfg.PartitionCountAlert)
	if err != nil {
		return err
	}
	compactionReadCostJob, err := compactionreadcost.NewJob(cfg.CompactionReadCostAlert)
	if err != nil {
		return err
	}
	workerLivenessJob, err := workerliveness.NewJob(cfg.WorkerLivenessAlert)
	if err != nil {
		return err
	}
	collectorProgressJob, err := collectorprogress.NewJob(cfg.MetricCollectorProgressAlert)
	if err != nil {
		return err
	}

	metricCollectorProvisioner, err := collector.NewMetricsCollectorProvisioner(a.ds, &collector.MetricCollectorConfig{
		PollRate: cfg.MetricCollector.PollRate,
	}, a.Logger)
	if err != nil {
		return err
	}

	registered, err := a.systemController.Register(ctx)
	if err != nil {
		return err
	}

	owner, err := common.NewSystemOwner(registered.Id)
	if err != nil {
		return err
	}
	for _, declarer := range a.systemDeclarers {
		if err := declarer.Declare(ctx, owner); err != nil {
			return err
		}
	}

	// registerStream, not RegisterStream -- the latter guards the __system. prefix
	if _, err := a.registerStream(ctx, metric.MetricStreamName, metricscontroller.StreamConfig()); err != nil {
		return err
	}
	if _, err := a.registerStream(ctx, alert.AlertStreamName, alertcontroller.StreamConfig()); err != nil {
		return err
	}
	if _, err := a.registerStream(ctx, schedule.ScheduleStreamName, schedulecontroller.StreamConfig()); err != nil {
		return err
	}

	for _, job := range []*alertcontroller.Job{partitionCountJob, compactionReadCostJob, workerLivenessJob, collectorProgressJob} {
		if _, err := a.scheduler.Register[alert.JobPayload](ctx, job.Name, schedule.ScheduleStreamName, job.Cron, job.Payload, &scheduler.SchedulerConfig{
			Concurrency: common.ConcurrencyExclusive,
		}); err != nil {
			return err
		}
	}

	// declared after the streams: the alert declarers resolve the schedules
	// stream to create their consumer groups and worker rows
	if err := metricCollectorProvisioner.Declare(ctx, owner); err != nil {
		return err
	}
	for _, declarer := range a.alertDeclarers {
		if err := declarer.Declare(ctx, owner); err != nil {
			return err
		}
	}
	return nil
}

// GetSystem returns the singleton system config. Returns
// migrate.ErrNotRegistered if RegisterSystem hasn't run.
func (a *MessageAdmin) GetSystem(ctx context.Context) (*system.System, error) {
	sys, err := a.systemController.Get(ctx)
	if err != nil {
		return nil, err
	}
	if sys == nil {
		return nil, migrate.ErrNotRegistered
	}
	return sys, nil
}

// MigrateSystem moves the system's tables to targetVersion.
// Returns an error ErrNotRegistered if RegisterSystem hasn't run.
func (a *MessageAdmin) MigrateSystem(ctx context.Context, targetVersion int64) error {
	owner, err := a.SystemOwner(ctx)
	if err != nil {
		return err
	}
	return a.migrateController.RunOnce(ctx, targetVersion, owner, systemMigrations.Registry)
}

// SystemMigrationVersion reads the version the system's tables are at.
// Returns ErrNotRegistered if RegisterSystem hasn't run.
func (a *MessageAdmin) SystemMigrationVersion(ctx context.Context) (int64, error) {
	owner, err := a.SystemOwner(ctx)
	if err != nil {
		return 0, err
	}
	return a.migrateController.SystemVersion(ctx, owner.SystemId)
}

// DestroySystem permanently deletes:
// - every registered stream and its messages
// - the system streams
// - schedules
// - consumer groups
// - workers
// - shared control-plane tables
//
// Returns stream.ErrDestroyDisabled unless MessageAdminConfig.AllowDestroy is set.
// Idempotent -- a system already destroyed (or never registered) resolves as
// a no-op, and a re-run after a partial failure resumes where it stopped.
//
// Unless options.Force is set:
//   - a worker instance is still live   -> system.ErrSystemLive
//   - a non-system stream is registered  -> system.ErrStreamsRegistered
func (a *MessageAdmin) DestroySystem(ctx context.Context, options *DestroyOptions) error {
	if !a.allowDestroy {
		return stream.ErrDestroyDisabled
	}
	if options == nil {
		options = &DestroyOptions{}
	}

	sys, err := a.systemController.Get(ctx)
	if err != nil {
		return err
	}

	// already destroyed -- the end state this call exists to produce holds
	if sys == nil {
		return nil
	}

	if !options.Force {
		if err := a.assertSystemIdle(ctx); err != nil {
			return err
		}
	}

	// each stream through the same delete path DestroyStream uses, keeping its
	// partition-drain safety against a still-writing producer
	streams, err := a.streamController.List(ctx)
	if err != nil {
		return err
	}
	for _, found := range streams {
		if err := a.streamController.Delete(ctx, found.Id, found.Name); err != nil {
			return err
		}
	}

	return a.systemController.Delete(ctx)
}

// assertSystemIdle is DestroySystem's guard: nothing is running against the
// schema, and no user stream would be taken with it.
func (a *MessageAdmin) assertSystemIdle(ctx context.Context) error {
	// a running manager or consumer heartbeats its worker instances
	workers, err := a.metricController.WorkerSnapshots(ctx)
	if err != nil {
		return err
	}
	for _, snapshot := range workers {
		if snapshot.LiveInstances > 0 {
			return system.ErrSystemLive.With("worker", snapshot.Name)
		}
	}

	streams, err := a.streamController.List(ctx)
	if err != nil {
		return err
	}
	var names []string
	for _, found := range streams {
		if !isReservedStreamName(found.Name) {
			names = append(names, found.Name)
		}
	}
	if len(names) > 0 {
		return system.ErrStreamsRegistered.With("streams", strings.Join(names, ", "))
	}
	return nil
}
