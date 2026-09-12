package sqlstreams

import (
	"context"
	"errors"

	"github.com/agentstax/sqlstreams/pkg/consumer"
	"github.com/agentstax/sqlstreams/pkg/systemmanager"
	"golang.org/x/sync/errgroup"
)

// ConsumerInstance is a registered consumer group. Consume runs its session
// and, when its context supports cancellation, the deployment's upkeep
// unless ClientConfig.DisableManager is set.
type ConsumerInstance[Message Versioned] struct {
	instance *consumer.ConsumerInstance[Message]

	manager    *systemmanager.SystemManager
	runManager bool
}

func newConsumerInstance[Message Versioned](instance *consumer.ConsumerInstance[Message], manager *systemmanager.SystemManager, runManager bool) (*ConsumerInstance[Message], error) {
	if instance == nil {
		return nil, errors.New("instance must not be nil")
	}
	if manager == nil {
		return nil, errors.New("manager must not be nil")
	}
	return &ConsumerInstance[Message]{instance: instance, manager: manager, runManager: runManager}, nil
}

// Consume blocks for the group's session; cancel ctx to start graceful shutdown.
// options may be nil for the defaults. LifecycleContext supplies a shutdown context.
//
// A context without cancellation returns ErrLifecycleContextNotCancellable
// unless options.DisableGracefulShutdown is set; that case runs no manager.
// A second Consume on the same instance returns ErrAlreadyConsuming.
//
// The manager runs beside the session unless ClientConfig.DisableManager is set.
// A manager error before its first claim tears the session down; later manager
// failures are logged and retried. Session exit stops the paired manager.
func (i *ConsumerInstance[Message]) Consume(ctx context.Context, consumerFunc ConsumerFunc[Message], options *ConsumeOptions) error {
	if !i.runManager {
		return i.instance.Consume(ctx, consumerFunc, options)
	}

	// Preserve the session's cancellation guard before deriving a cancellable
	// context; the explicit opt-out runs only the consumer.
	if ctx.Done() == nil {
		return i.instance.Consume(ctx, consumerFunc, options)
	}

	// the session owns the pairing: whenever it returns -- nil included --
	// the manager stops; a manager error cancels runCtx so the session
	// drains too
	group, runCtx := errgroup.WithContext(ctx)
	managerCtx, stopManager := context.WithCancel(runCtx)
	group.Go(func() error {
		defer stopManager()
		return i.instance.Consume(runCtx, consumerFunc, options)
	})
	group.Go(func() error {
		return i.manager.Run(managerCtx)
	})
	return group.Wait()
}
