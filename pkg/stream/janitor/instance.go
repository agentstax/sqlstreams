package janitor

import (
	"context"
	"errors"
	"fmt"

	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/common/logging"
	"github.com/agentstax/sqlstreams/pkg/stream"
	janitorcontroller "github.com/agentstax/sqlstreams/pkg/stream/janitor/controller"
	"github.com/agentstax/sqlstreams/pkg/worker"
	"github.com/agentstax/sqlstreams/pkg/worker/controller"
)

// sweeps the stream at the row's poll_rate while a heartbeat holds the claim
type JanitorInstance struct {
	Stream *stream.Stream
	Config *JanitorConfig
	Logger logging.Logger

	runner     *controller.InstanceTickRunner
	controller *janitorcontroller.JanitorController
	metadata   *janitorMetadata
}

func newJanitorInstance(janitor *JanitorProvisioner, current *stream.Stream, claimed *worker.WorkerInstance, metadata *janitorMetadata) (*JanitorInstance, error) {
	if current == nil {
		return nil, errors.New("stream must not be nil")
	}
	if metadata == nil {
		return nil, errors.New("metadata must not be nil")
	}

	logger := logging.NewPipelineLogger(janitor.Logger, &logging.PipelineLoggerConfig{Args: []any{"worker", WorkerStreamJanitor, "stream_id", current.Id}})
	runner, err := controller.NewInstanceTickRunner(janitor.workers, claimed, metadata.PollRate, &controller.InstanceTickRunnerConfig{
		InstanceTTL:    janitor.Config.InstanceTTL,
		JitterFraction: janitor.Config.JitterFraction,
		TickRetry:      janitor.Config.SweepRetry,
	}, logger)
	if err != nil {
		return nil, err
	}

	return &JanitorInstance{
		Stream:     current,
		Config:     janitor.Config,
		Logger:     logger,
		runner:     runner,
		controller: janitor.controller,
		metadata:   metadata,
	}, nil
}

// Run sweeps until ctx cancels; a requested stop returns nil. The claimed
// instance releases on the way out however Run exits.
func (i *JanitorInstance) Run(ctx context.Context) error {
	i.Logger.InfoContext(ctx, "janitor starting", "sqlstreams_version", common.BuildVersion(), "rate", i.metadata.PollRate, "cleanup_timeout", i.Config.CleanupTimeout)

	err := i.runner.Run(ctx, i.sweep)
	if err == nil {
		i.Logger.InfoContext(ctx, "janitor stopped")
	}
	return err
}

// sweep is one janitor pass.
func (i *JanitorInstance) sweep(ctx context.Context) error {
	current := i.Stream
	var sweepErrors []error

	if err := ctx.Err(); err != nil {
		return err
	}

	dropCtx, cancelDrop := context.WithTimeout(ctx, i.Config.CleanupTimeout)
	err := i.controller.DropExpiredPartitions(dropCtx, current.Id, current.PartitionSize, current.RetentionTTL, current.AllowDropPastCommitted, current.DeliveryLogMode)
	cancelDrop()
	if err != nil {
		sweepErrors = append(sweepErrors, fmt.Errorf("drop expired partitions: %w", err))
	}

	if err := ctx.Err(); err != nil {
		return errors.Join(append(sweepErrors, err)...)
	}

	partitionsCtx, cancelPartitions := context.WithTimeout(ctx, i.Config.CleanupTimeout)
	err = i.controller.SweepExpiredPartitions(partitionsCtx, current.Id, current.PartitionSize, current.RetentionTTL, current.AllowDropPastCommitted, i.metadata.SweepBatchSize, current.DeliveryLogMode)
	cancelPartitions()
	if err != nil {
		sweepErrors = append(sweepErrors, fmt.Errorf("sweep expired partitions: %w", err))
	}

	if err := ctx.Err(); err != nil {
		return errors.Join(append(sweepErrors, err)...)
	}

	idempotencyKeysCtx, cancelIdempotencyKeys := context.WithTimeout(ctx, i.Config.CleanupTimeout)
	err = i.controller.SweepExpiredIdempotencyKeys(idempotencyKeysCtx, current.Id, current.IdempotencyKeyTTL, i.metadata.SweepBatchSize)
	cancelIdempotencyKeys()
	if err != nil {
		sweepErrors = append(sweepErrors, fmt.Errorf("sweep expired idempotency keys: %w", err))
	}

	if err := ctx.Err(); err != nil {
		return errors.Join(append(sweepErrors, err)...)
	}

	emptyCompactionHeadsCtx, cancelEmptyCompactionHeads := context.WithTimeout(ctx, i.Config.CleanupTimeout)
	err = i.controller.SweepExpiredEmptyCompactionHeads(emptyCompactionHeadsCtx, current.Id, current.EmptyCompactionHeadTTL, i.metadata.SweepBatchSize)
	cancelEmptyCompactionHeads()
	if err != nil {
		sweepErrors = append(sweepErrors, fmt.Errorf("sweep expired empty compaction heads: %w", err))
	}

	if err := ctx.Err(); err != nil {
		return errors.Join(append(sweepErrors, err)...)
	}

	keyLeasesCtx, cancelKeyLeases := context.WithTimeout(ctx, i.Config.CleanupTimeout)
	err = i.controller.SweepExpiredKeyLeases(keyLeasesCtx, current.Id, i.metadata.SweepBatchSize)
	cancelKeyLeases()
	if err != nil {
		sweepErrors = append(sweepErrors, fmt.Errorf("sweep expired key leases: %w", err))
	}

	return errors.Join(sweepErrors...)
}
