package scheduler

import (
	"context"
	"errors"
	"time"

	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/produce"
	"github.com/agentstax/sqlstreams/pkg/producer"
	"github.com/agentstax/sqlstreams/pkg/schedule"
	schedulecontroller "github.com/agentstax/sqlstreams/pkg/schedule/controller"
	"github.com/agentstax/sqlstreams/pkg/stream"
	streamcontroller "github.com/agentstax/sqlstreams/pkg/stream/controller"
)

// RunSchedule produces the named schedule's stored message immediately,
// outside its expression -- the expression and next scheduled time are
// untouched, and a suspended schedule still runs.
// options may be nil for the defaults.
// Returns schedule.ErrScheduleNotFound if name isn't registered.
//
// Two deliberate consequences:
//   - The message's concurrency is options.Concurrency, NOT the schedule's own
//     policy -- by default 'parallel', so it runs even while a previous
//     message is still running.
//   - It supersedes a pending message no consumer has claimed yet.
func (s *Scheduler) RunSchedule(ctx context.Context, name string, options *ScheduleRunOptions) (*producer.ProduceResult[schedule.ScheduleStoredMessage], error) {
	if name == "" {
		return nil, errors.New("schedule name is required")
	}
	if options == nil {
		options = &ScheduleRunOptions{}
	}
	options.WithDefaults()
	if err := options.Validate(); err != nil {
		return nil, err
	}

	scheduleController, err := schedulecontroller.NewScheduleController(s.ds, s.ds.Logger)
	if err != nil {
		return nil, err
	}
	streamController, err := streamcontroller.NewStreamController(s.ds, s.ds.Logger)
	if err != nil {
		return nil, err
	}
	scheduleProducer, err := producer.NewProducer(s.ds)
	if err != nil {
		return nil, err
	}

	found, err := scheduleController.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, schedule.ErrScheduleNotFound.With("schedule", name)
	}

	target, err := streamController.GetById(ctx, found.StreamId)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, stream.ErrStreamNotFound.With("stream_id", found.StreamId)
	}
	instance, err := scheduleProducer.Register[schedule.ScheduleStoredMessage](ctx, target.Name, nil)
	if err != nil {
		return nil, err
	}

	stored, err := schedule.NewScheduleStoredMessage(found.Payload, found.SchemaVersion)
	if err != nil {
		return nil, err
	}

	// no IdempotencyKey: Produce creates a fresh v7 per call, so every run is
	// its own message
	return instance.Produce(ctx, stored, &produce.ProduceOptions{
		RoutingKey: found.Name,
		MessageKey: found.Name,
		Compaction: &produce.CompactionOptions{Enable: true},
		Message: &common.MessageOptions{
			Concurrency: options.Concurrency,
			Timeout:     found.Timeout,
			ScheduledAt: time.Now().UTC(),
		},
	})
}
