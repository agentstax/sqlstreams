package batcher

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/common/logging"
	"github.com/agentstax/sqlstreams/pkg/produce"
	"github.com/agentstax/sqlstreams/pkg/produce/controller"
)

// Batcher groups concurrent payload-only produces for one stream into shared
// transactions, amortizing the per-commit fsync in the database.
type Batcher[Message common.Versioned] struct {
	Config *BatcherConfig
	Logger logging.Logger

	controller    *controller.ProduceController
	streamId      int64
	partitionSize int64

	queue workQueue[batchOperation[Message]]
}

// cfg may be nil or sparse. logger is the owning
// producer instance's.
func NewBatcher[Message common.Versioned](produceController *controller.ProduceController, streamId int64, partitionSize int64, cfg *BatcherConfig, logger logging.Logger) (*Batcher[Message], error) {
	if produceController == nil {
		return nil, errors.New("controller must not be nil")
	}
	if streamId <= 0 {
		return nil, fmt.Errorf("streamId must be > 0, got %d", streamId)
	}
	if cfg == nil {
		cfg = &BatcherConfig{}
	}
	cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	return &Batcher[Message]{
		Config:        cfg,
		Logger:        logger,
		controller:    produceController,
		streamId:      streamId,
		partitionSize: partitionSize,
	}, nil
}

// Produce enqueues one message and blocks until its batch commits (durable) or fails.
func (b *Batcher[Message]) Produce(ctx context.Context, message *Message, options produce.ProduceOptions) (*controller.Appended[Message], error) {
	// already cancelled -> fail BEFORE enqueue. This is the graceful shutdown path:
	// a cancelled producer refuses new work while enqueued work resolves.
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("produce rejected before enqueue for stream %d, nothing was published: %w", b.streamId, err)
	}

	// always minted fresh -- fresh V7 keys cannot collide inside the shared txn
	options.IdempotencyKey = uuid.NewV7().String()

	operation := newBatchOperation(message, options)

	b.queue.enqueue(operation)
	if b.queue.needsWorker(b.Config.MaxSize, b.Config.ConcurrencyLimit) {
		go b.work()
	}

	select {
	case <-operation.response.done:
		// continue past select
	case <-ctx.Done():
		// exit early with no shutdownGrace
		if b.Config.ShutdownGrace < 0 {
			return nil, fmt.Errorf("produce abandoned for stream %d, batch outcome ambiguous (ShutdownGrace < 0): %w", b.streamId, ctx.Err())
		}

		// enqueued work cannot be recalled -- wait up to the grace for the
		// real outcome before abandoning as ambiguous
		grace := time.NewTimer(b.Config.ShutdownGrace)
		defer grace.Stop()

		select {
		case <-operation.response.done:
			// ideally this completes -> graceful shutdown
			b.Logger.DebugContext(ctx, "cancelled produce resolved within shutdown grace", "stream_id", b.streamId)
		case <-grace.C:
			// if shutdownGrace times out -> exit early
			// work commit status is ambiguous and should be retried if possible when supplying external idempotency key
			return nil, fmt.Errorf("produce abandoned after ShutdownGrace (%s) for stream %d, batch outcome ambiguous: %w", b.Config.ShutdownGrace, b.streamId, ctx.Err())
		}
	}

	if err := operation.response.err; err != nil {
		return nil, err
	}
	appended := operation.response.appended
	return &appended, nil
}

// work fires batches one after the other until the queue empties.
func (b *Batcher[Message]) work() {
	// the worker's own context, never a caller's: a caller cancelling stops
	// waiting, it must not abort a transaction other operations share
	ctx := context.Background()
	for {
		operations := b.queue.dequeue(b.Config.MaxSize)
		if operations == nil {
			return
		}
		b.resolveBatch(ctx, newBatch(operations))
	}
}
