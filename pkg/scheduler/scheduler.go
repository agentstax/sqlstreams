package scheduler

import (
	"context"
	"errors"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/datastore"
	"github.com/allegedlyreliable/sqlstreams/pkg/migrate"
	"github.com/allegedlyreliable/sqlstreams/pkg/schedule"
	schedulecontroller "github.com/allegedlyreliable/sqlstreams/pkg/schedule/controller"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
	streamcontroller "github.com/allegedlyreliable/sqlstreams/pkg/stream/controller"
	systemcontroller "github.com/allegedlyreliable/sqlstreams/pkg/system/controller"
)

// Scheduler declares schedules; the system's schedule producer worker is what produces them.
type Scheduler struct {
	ds *datastore.PostgresDatastore
}

// NewScheduler builds the datastore-only registration object. Register owns
// the config because each call returns an independently configured instance.
func NewScheduler(ds *datastore.PostgresDatastore) (*Scheduler, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	return &Scheduler{ds: ds}, nil
}

// Register declares the named schedule on its target stream and returns an
// instance for it. Safe to call on every startup: the newest registration
// wins, so two services passing different values for one name overwrite
// each other. A changed expression drops a time already due under the old
// one; a suspended schedule stays suspended. The name is the message key of
// every produce. cfg may be nil or sparse.
// ctx bounds only this call's I/O; the instance's lifetime is Schedule's ctx.
func (s *Scheduler) Register[Message common.Versioned](ctx context.Context, name string, streamName string, cron string, payload *Message, cfg *SchedulerConfig) (*SchedulerInstance[Message], error) {
	if name == "" {
		return nil, errors.New("schedule name is required")
	}
	if streamName == "" {
		return nil, errors.New("stream name is required")
	}
	if cfg == nil {
		cfg = &SchedulerConfig{}
	}
	cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	systemController, err := systemcontroller.NewSystemController(s.ds, s.ds.Logger)
	if err != nil {
		return nil, err
	}
	streamController, err := streamcontroller.NewStreamController(s.ds, s.ds.Logger)
	if err != nil {
		return nil, err
	}
	scheduleController, err := schedulecontroller.NewScheduleController(s.ds, s.ds.Logger)
	if err != nil {
		return nil, err
	}

	// gate -- a schedule needs the control-plane tables RegisterSystem creates
	sys, err := systemController.Get(ctx)
	if err != nil {
		return nil, err
	}
	if sys == nil {
		return nil, migrate.ErrNotRegistered.With("schedule", name)
	}

	target, err := streamController.Get(ctx, streamName)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, stream.ErrStreamNotFound.With("stream", streamName)
	}
	if err := streamController.AssertSchemaSupported(ctx, target.SystemId, target.Id); err != nil {
		return nil, err
	}

	if target.DeliveryLogMode != stream.DeliveryLogModeAll {
		s.ds.Logger.WarnContext(ctx, schedule.EventTargetKeepsNoSuccessRows.Message(), "code", schedule.EventTargetKeepsNoSuccessRows.GetCode(), "schedule", name, "stream", target.Name, "delivery_log_mode", string(target.DeliveryLogMode))
	}

	registered, err := scheduleController.Register(ctx, sys.Id, name, cron, target.Id, payload, cfg.Timeout, cfg.Concurrency, cfg.Metadata)
	if err != nil {
		return nil, err
	}

	return newSchedulerInstance[Message](registered, payload, s.ds, cfg)
}
