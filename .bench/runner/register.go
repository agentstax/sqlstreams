package runner

import (
	"context"

	"github.com/agentstax/sqlstreams/.bench/common"
	"github.com/agentstax/sqlstreams/.bench/scenario"
	"github.com/agentstax/sqlstreams/client"
)

// The scenario's [input] section, translated into the library's own
// declarations. Every role registers: registration is idempotent and
// newest-wins, so whichever role starts first bootstraps and the rest agree.

// registeredStream is one declared stream and its handle, in declaration
// order.
type registeredStream struct {
	declared scenario.StreamDeclaration
	handle   *sqlstreams.StreamHandle[common.Order]
}

func (r *Runner) registerStreams(ctx context.Context) ([]registeredStream, error) {
	if err := r.connection.Client.System().Register(ctx, nil); err != nil {
		return nil, err
	}
	registered := make([]registeredStream, 0, len(r.declared.Streams))
	for _, declared := range r.declared.Streams {
		handle := r.connection.Client.Stream[common.Order](declared.Name)
		if _, err := handle.Register(ctx, &sqlstreams.StreamConfig{DeliveryLogMode: declared.DeliveryLogMode, PartitionSize: declared.PartitionSize, RetentionTTL: declared.RetentionTTL, IdempotencyKeyTTL: declared.IdempotencyKeyTTL, Janitor: declared.Janitor, Vacuum: declared.Vacuum}); err != nil {
			return nil, err
		}
		if declared.VacuumEnabled {
			if err := handle.Vacuum().Unsuspend(ctx); err != nil {
				return nil, err
			}
		}
		registered = append(registered, registeredStream{declared: declared, handle: handle})
	}
	if r.declared.DisableExceptionConsumers {
		for _, registered := range registered {
			for _, group := range registered.declared.Groups {
				if _, err := registered.handle.Consumer(group.Name).Register(ctx, consumerConfig(group)); err != nil {
					return nil, err
				}
			}
		}
		if err := r.ds.SuspendExceptionConsumers(ctx); err != nil {
			return nil, err
		}
	}
	return registered, nil
}

// producerConfig is the producer line; a zero batch concurrency is the
// library's own default.
func producerConfig(declared *scenario.Scenario) *sqlstreams.ProducerConfig {
	cfg := &sqlstreams.ProducerConfig{}
	cfg.Batch.ConcurrencyLimit = declared.ProducerBatchConcurrency
	cfg.Batch.MaxSize = declared.ProducerBatchSize
	return cfg
}

// consumerConfig is the "N retries then dead" half of a consumers line.
func consumerConfig(group scenario.GroupDeclaration) *sqlstreams.ConsumerConfig {
	return (&sqlstreams.ConsumerConfig{
		Message: &sqlstreams.MessageOptions{
			Retry: &sqlstreams.RetryPolicy{MaxRetries: group.MaxRetries},
		},
	}).WithDefaults()
}

// consumeOptions is the "batch N" half; a zero BatchLimit is the library's
// own default.
func consumeOptions(group scenario.GroupDeclaration) *sqlstreams.ConsumeOptions {
	return &sqlstreams.ConsumeOptions{BatchLimit: group.BatchLimit, QueueSize: group.QueueSize, MessageConcurrency: group.MessageConcurrency, ClaimPollRate: group.ClaimPollRate}
}
