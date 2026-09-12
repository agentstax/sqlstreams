package base

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/common/logging"
	"github.com/allegedlyreliable/sqlstreams/pkg/consume"
	"github.com/allegedlyreliable/sqlstreams/pkg/consume/base/controller"
	metricsproducer "github.com/allegedlyreliable/sqlstreams/pkg/metric/producer"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
	workercontroller "github.com/allegedlyreliable/sqlstreams/pkg/worker/controller"
)

// BaseConsumer is built fresh per claimed life, so a respawned runner never
// shares state with a predecessor still draining.
type BaseConsumer[Message common.Versioned] struct {
	Owner         *common.Owner
	Stream        *stream.Stream
	SchemaVersion int
	Config        *BaseConsumerConfig
	Logger        logging.Logger
	Metrics       *metricsproducer.MetricProducer
	Workers       *workercontroller.WorkerController
	KeyLeases     *controller.KeyLeaseController

	consumerFunc func(ctx context.Context, message *Message) error
}

// resolvedStream comes from BaseProvisioner.GetStream. cfg may be nil or
// sparse.
func NewBaseConsumer[Message common.Versioned](baseProvisioner *BaseProvisioner[Message], owner *common.Owner, resolvedStream *stream.Stream, cfg *BaseConsumerConfig) (*BaseConsumer[Message], error) {
	if baseProvisioner == nil {
		return nil, errors.New("provisioner base must not be nil")
	}
	if owner == nil {
		return nil, errors.New("owner must not be nil")
	}
	if resolvedStream == nil {
		return nil, errors.New("stream must not be nil")
	}
	if cfg == nil {
		cfg = &BaseConsumerConfig{}
	}
	cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &BaseConsumer[Message]{
		Owner:         owner,
		Stream:        resolvedStream,
		SchemaVersion: baseProvisioner.schemaVersion,
		Config:        cfg,
		Logger:        baseProvisioner.Logger,
		Metrics:       baseProvisioner.metrics,
		Workers:       baseProvisioner.workers,
		KeyLeases:     baseProvisioner.keyLeases,
		consumerFunc:  baseProvisioner.consumerFunc,
	}, nil
}

// Handles: nil map write, index out of range, bad type assertion
// Does not handle: OS-level fault -- stack overflow, SIGSEGV via cgo, OOM-kill, external kill
func (b *BaseConsumer[Message]) CallSafely(ctx context.Context, payload *Message, messageId int64, attempt int, requested *common.MessageOptions, timeout time.Duration) error {
	defer b.warnSlowDispatch(ctx, time.Now(), messageId, attempt)
	ctx = logging.WithLogBuffer(ctx)

	// the timeout cause names which side's budget fired
	cause := fmt.Errorf("Timeout (%s) exceeded for message %d attempt %d", timeout, messageId, attempt)
	if requested != nil && requested.Timeout > timeout {
		cause = fmt.Errorf("Timeout (%s) exceeded for message %d attempt %d -- message requested %s, the group ceiling applied", timeout, messageId, attempt, requested.Timeout)
	}

	// work should not be immediately cancelled on a SIGINT/SIGTERM (cancel or shutdown)
	// instead attempt to finish inflight requests bounded by timeout
	ctx, cancel := context.WithTimeoutCause(context.WithoutCancel(ctx), timeout, cause)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// done is buffered -- always deliverable, even if the select below
				// already gave up on this goroutine via the timeout branch.
				done <- fmt.Errorf("recovered from consumerFunc panic: %v\n%s", r, debug.Stack())
			}
		}()
		done <- b.consumerFunc(ctx, payload)
	}()

	select {
	case err := <-done:
		return err
	// consumerFunc got Timeout to notice ctx and return; past Timeout + grace it
	// is written off and its goroutine is left running, unreachable
	case <-time.After(timeout + b.Config.TimeoutGrace):
		b.Metrics.RecordAbandoned(b.Stream.Id, b.Owner.Name, messageId, attempt)

		// done is buffered(1) and nothing else reads it past this point, so this
		// receive fires exactly when the abandoned goroutine finally returns.
		// Started after Add, so Remove can never precede it.
		go func() {
			<-done
			b.Metrics.RecordCleared(b.Stream.Id, b.Owner.Name, messageId, attempt)
		}()

		// never include the message itself -- it may hold sensitive values
		return fmt.Errorf("hard timeout after %s, goroutine abandoned for message %d", timeout+b.Config.TimeoutGrace, messageId)
	}
}

// warnSlowDispatch logs one line when a delivery's dispatch ran past the
// configured threshold -- slowness is its own fact, logged whatever the
// delivery's outcome.
func (b *BaseConsumer[Message]) warnSlowDispatch(ctx context.Context, start time.Time, messageId int64, attempt int) {
	duration := time.Since(start)
	if b.Config.SlowDispatchThreshold <= 0 || duration <= b.Config.SlowDispatchThreshold {
		return
	}
	b.Logger.WarnContext(ctx, consume.EventSlowDispatch.Message(), "code", consume.EventSlowDispatch.GetCode(), "group", b.Owner.Name, "stream_id", b.Stream.Id, "message_id", messageId, "attempt", attempt, "duration", duration, "threshold", b.Config.SlowDispatchThreshold)
}
