package consumer

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/common/concurrency"
	"github.com/agentstax/sqlstreams/pkg/common/logging"
	"github.com/agentstax/sqlstreams/pkg/consume"
	consumecontroller "github.com/agentstax/sqlstreams/pkg/consume/controller"
	"github.com/agentstax/sqlstreams/pkg/datastore"
	metricsproducer "github.com/agentstax/sqlstreams/pkg/metric/producer"
	"golang.org/x/sync/errgroup"
)

// ConsumerInstance is a registered consumer group: Consume runs its manager,
// which spawns and heals every worker in the group's chain.
type ConsumerInstance[Message common.Versioned] struct {
	Owner  *common.Owner   // the group's identity: stream id, group id, and name
	Config *ConsumerConfig // the declaration Register resolved -- what the group means
	Logger logging.Logger  // bound to the group; every worker in the chain logs through it

	ds            *datastore.PostgresDatastore
	metrics       *metricsproducer.MetricProducer
	consumers     *consumecontroller.ConsumeController
	streamName    string
	streamVersion int
	declaredAt    time.Time
	permit        *concurrency.Permit // held for the length of a Consume call
}

// cfg arrives already resolved by Register, the only caller, so there is
// nothing left to default or validate here; logger is Register's
// per-instance pipeline over the datastore's logger.
// declaredAt is Register's declaration time; Consume re-attempts the
// Config.Bindings declaration under it.
func newConsumerInstance[Message common.Versioned](owner *common.Owner, ds *datastore.PostgresDatastore, metrics *metricsproducer.MetricProducer, consumers *consumecontroller.ConsumeController, streamName string, streamVersion int, declaredAt time.Time, cfg *ConsumerConfig, logger logging.Logger) (*ConsumerInstance[Message], error) {
	if owner == nil {
		return nil, errors.New("owner must not be nil")
	}
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if metrics == nil {
		return nil, errors.New("metrics must not be nil")
	}
	if consumers == nil {
		return nil, errors.New("consumers must not be nil")
	}
	if streamName == "" {
		return nil, errors.New("streamName must not be empty")
	}
	if declaredAt.IsZero() {
		return nil, errors.New("declaredAt is required")
	}
	if cfg == nil {
		return nil, errors.New("config must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	permit, err := concurrency.NewPermit()
	if err != nil {
		return nil, err
	}

	return &ConsumerInstance[Message]{
		Owner:         owner,
		Config:        cfg,
		Logger:        logger,
		ds:            ds,
		metrics:       metrics,
		consumers:     consumers,
		streamName:    streamName,
		streamVersion: streamVersion,
		declaredAt:    declaredAt,
		permit:        permit,
	}, nil
}

// Consume blocks until stopped: ctx is the instance's lifetime, cancel it to
// shut down in-flight work and return nil. A runner's fatal error tears the
// instance down and returns here. ctx must be cancellable, unless
// ConsumeOptions.DisableGracefulShutdown declares otherwise.
// options may be nil for the defaults.
//
// Returns ErrLifecycleContextNotCancellable for a ctx that can never be
// cancelled, and ErrAlreadyConsuming while another Consume on this instance
// is still running -- one session per instance.
func (i *ConsumerInstance[Message]) Consume(ctx context.Context, consumerFunc ConsumerFunc[Message], options *ConsumeOptions) error {
	if consumerFunc == nil {
		return errors.New("consumerFunc must not be nil")
	}

	resolved := ConsumeOptions{}
	if options != nil {
		resolved = *options
	}
	resolved.WithDefaults()
	if err := resolved.Validate(); err != nil {
		return err
	}

	// ShutdownTimeout stays sparse so the drain can re-derive it when a
	// config refresh moves the group's ceiling; this session-start value
	// exists only for the start line below
	shutdownTimeout := resolved.ShutdownTimeout
	if shutdownTimeout == 0 {
		shutdownTimeout = i.Config.MessageMax.Timeout + resolved.TimeoutGrace + resolved.RecordMargin
	}

	// Done() == nil -> Background/TODO -> no cancel can ever arrive, so the
	// shutdown phase would silently not exist
	if ctx.Done() == nil && !resolved.DisableGracefulShutdown {
		return fmt.Errorf("%w\n%s", common.ErrLifecycleContextNotCancellable.With("group", i.Owner.Name), lifecycleContextHelp)
	}

	release, ok := i.permit.Acquire()
	if !ok {
		return common.ErrAlreadyConsuming.With("group", i.Owner.Name, "stream_id", i.Owner.StreamId)
	}
	defer release()

	// blocking until bindings install or join
	if err := i.declareBindings(ctx, resolved.BindingRetryInterval); err != nil {
		// a cancel during the wait is a requested stop, not a failure
		if ctx.Err() != nil {
			return nil
		}
		return err
	}

	runner, err := i.newManagerRunner(ctx, consumerFunc, &resolved)
	if err != nil {
		return err
	}

	// a Consume call is one session: counters restart at zero and the flushed
	// series gets a fresh identity
	session := uuid.NewV7()
	i.metrics.ResetCounters()

	i.Logger.InfoContext(ctx, "consumer starting", "group", i.Owner.Name, "stream_id", i.Owner.StreamId, "sqlstreams_version", common.BuildVersion(), "message_timeout", i.Config.Message.Timeout, "shutdown_timeout", shutdownTimeout, "batch_limit", resolved.BatchLimit, "queue_size", resolved.QueueSize, "message_concurrency", resolved.MessageConcurrency, "claim_poll_rate", resolved.ClaimPollRate, "queue_margin", resolved.QueueMargin, "message_max_timeout", i.Config.MessageMax.Timeout, "timeout_grace", resolved.TimeoutGrace, "record_margin", resolved.RecordMargin)
	started := time.Now()

	group, runCtx := errgroup.WithContext(ctx)

	group.Go(func() error {
		return i.metrics.Run(runCtx, i.Owner.Name, i.streamName, i.streamVersion, session.String())
	})
	group.Go(func() error {
		return runner.Run(runCtx)
	})

	err = group.Wait()

	// every exit gets the summary -- a failed session is the one whose
	// numbers matter most. The error itself is returned, never logged.
	i.logStopped(context.WithoutCancel(ctx), started)
	return err
}

// logStopped is the session summary: bound identity, session wall time, and
// every session counter, zeros included.
func (i *ConsumerInstance[Message]) logStopped(ctx context.Context, started time.Time) {
	counters := i.metrics.Snapshot()
	i.Logger.InfoContext(ctx, consume.EventConsumerStopped.Message(),
		"code", consume.EventConsumerStopped.GetCode(),
		"group", i.Owner.Name,
		"stream_id", i.Owner.StreamId,
		"duration", time.Since(started),
		"claimed_count", counters.Claimed,
		"success_count", counters.Success,
		"superseded_count", counters.Superseded,
		"ready_count", counters.Ready,
		"deferred_count", counters.Deferred,
		"dead_count", counters.Dead,
		"reclaimed_count", counters.Reclaimed,
		"quarantined_count", counters.Quarantined,
		"abandoned_count", counters.Abandoned,
		"lease_lost_count", counters.LeaseLost,
		"help", "metrics explained: sqlstreams explain "+consume.EventConsumerStopped.GetCode())
}

// declareBindings retries the declaration until it is installed or joined.
func (i *ConsumerInstance[Message]) declareBindings(ctx context.Context, bindingRetryInterval time.Duration) error {
	for attempt := 1; ; attempt++ {
		// Register's outcome is not trusted -- another declarer may have
		// replaced the set while this instance had no live heartbeat
		outcome, err := i.consumers.DeclareBindings(ctx, i.Owner.StreamId, i.Owner.ConsumerGroupId, i.Config.Bindings, i.declaredAt)
		if err != nil {
			return err
		}
		if outcome != consume.BindingWaiting {
			return nil
		}

		i.Logger.WarnContext(ctx, "binding declaration waiting -- a live instance still declares a different set",
			"group", i.Owner.Name,
			"patterns", i.Config.Bindings,
			"attempt", attempt,
			"elapsed", time.Since(i.declaredAt).Round(time.Second))

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(bindingRetryInterval):
		}
	}
}
