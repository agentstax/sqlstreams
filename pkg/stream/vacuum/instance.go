package vacuum

import (
	"context"
	"errors"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/common/logging"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
	vacuumcontroller "github.com/allegedlyreliable/sqlstreams/pkg/stream/vacuum/controller"
	"github.com/allegedlyreliable/sqlstreams/pkg/worker"
	"github.com/allegedlyreliable/sqlstreams/pkg/worker/controller"
)

type VacuumInstance struct {
	Stream *stream.Stream
	Config *VacuumConfig
	Logger logging.Logger

	runner     *controller.InstanceTickRunner
	controller *vacuumcontroller.VacuumController
	metadata   *vacuumMetadata
}

func newVacuumInstance(vacuum *VacuumProvisioner, current *stream.Stream, claimed *worker.WorkerInstance, metadata *vacuumMetadata) (*VacuumInstance, error) {
	if current == nil {
		return nil, errors.New("stream must not be nil")
	}
	if metadata == nil {
		return nil, errors.New("metadata must not be nil")
	}

	logger := logging.NewPipelineLogger(vacuum.Logger, &logging.PipelineLoggerConfig{Args: []any{"worker", WorkerStreamVacuum, "stream_id", current.Id}})
	runner, err := controller.NewInstanceTickRunner(vacuum.workers, claimed, metadata.PollRate, &controller.InstanceTickRunnerConfig{
		InstanceTTL:    vacuum.Config.InstanceTTL,
		JitterFraction: vacuum.Config.JitterFraction,
		TickRetry:      vacuum.Config.VacuumRetry,
	}, logger)
	if err != nil {
		return nil, err
	}

	return &VacuumInstance{
		Stream:     current,
		Config:     vacuum.Config,
		Logger:     logger,
		runner:     runner,
		controller: vacuum.controller,
		metadata:   metadata,
	}, nil
}

// Run vacuums until ctx cancels; a requested stop returns nil. The claimed
// instance releases on the way out however Run exits.
func (i *VacuumInstance) Run(ctx context.Context) error {
	started := time.Now()
	defer func() {
		i.Logger.InfoContext(ctx, "vacuum stopped", "duration", time.Since(started))
	}()
	i.Logger.InfoContext(ctx, "vacuum starting", "sqlstreams_version", common.BuildVersion(), "rate", i.metadata.PollRate, "vacuum_timeout", i.metadata.VacuumTimeout, "jitter_fraction", i.Config.JitterFraction)

	return i.runner.Run(ctx, i.vacuum)
}

func (i *VacuumInstance) vacuum(ctx context.Context) error {
	vacuumCtx, cancel := context.WithTimeout(ctx, i.metadata.VacuumTimeout)
	defer cancel()

	return i.controller.VacuumIdempotencyKeys(vacuumCtx, i.Stream.Id)
}
