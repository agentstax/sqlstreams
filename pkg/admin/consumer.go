package admin

import (
	"context"
	"errors"

	"github.com/allegedlyreliable/sqlstreams/pkg/consume"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
	"github.com/allegedlyreliable/sqlstreams/pkg/worker"
)

// GetConsumer reads the consumer's registration. Returns (nil, nil), not an error, when
// the stream or the group isn't registered.
func (a *MessageAdmin) GetConsumer(ctx context.Context, streamName string, consumerName string) (*consume.Consumer, error) {
	if consumerName == "" {
		return nil, errors.New("consumer name is required")
	}

	found, err := a.GetStream(ctx, streamName)
	if err != nil || found == nil {
		return nil, err
	}
	return a.consumerController.GetGroup(ctx, found.Id, consumerName)
}

// ListConsumers lists the stream's registered consumers, ordered by name.
// Returns ErrStreamNotFound when the stream isn't registered.
func (a *MessageAdmin) ListConsumers(ctx context.Context, streamName string) ([]*consume.Consumer, error) {
	found, err := a.GetStream(ctx, streamName)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, stream.ErrStreamNotFound.With("stream", streamName)
	}
	return a.consumerController.ListGroups(ctx, found.Id)
}

// ListConsumerWorkers lists the consumer group's shared worker rows -- its stored config.
// Returns ErrStreamNotFound / ErrConsumerNotFound when either side is missing.
func (a *MessageAdmin) ListConsumerWorkers(ctx context.Context, streamName string, consumerName string) ([]*worker.Worker, error) {
	consumerGroupOwner, err := a.ConsumerGroupOwner(ctx, streamName, consumerName)
	if err != nil {
		return nil, err
	}
	return a.workerController.ListConsumerGroupWorkers(ctx, consumerGroupOwner)
}

// DestroyConsumer permanently deletes the consumer group registered under
// consumerName on streamName: its cursor, bindings, leases,
// delivery rows, group-owned workers, and group-owned schedules. The
// stream and its messages are untouched. All instances share this registration.
//
// Returns stream.ErrDestroyDisabled unless MessageAdminConfig.AllowDestroy is set,
// and ErrStreamNotFound / ErrConsumerNotFound when either side is missing.
// Unless options.Force is set:
//   - any consumer instance is live     -> ErrConsumerGroupLive
//   - the group still holds delivery rows    -> ErrConsumerGroupDeliveriesPending
func (a *MessageAdmin) DestroyConsumer(ctx context.Context, streamName string, consumerName string, options *DestroyOptions) error {
	if !a.allowDestroy {
		return stream.ErrDestroyDisabled
	}
	if options == nil {
		options = &DestroyOptions{}
	}

	owner, err := a.ConsumerGroupOwner(ctx, streamName, consumerName)
	if err != nil {
		return err
	}

	if !options.Force {
		if err := a.assertConsumerGroupIdle(ctx, owner.StreamId, owner.ConsumerGroupId, owner.Name); err != nil {
			return err
		}
	}

	return a.consumerController.DeleteGroup(ctx, owner.StreamId, owner.ConsumerGroupId, owner.Name)
}

// assertConsumerGroupIdle is DestroyConsumer's guard: nothing is consuming on the
// group and no delivery rows would be discarded. Both facts come from the
// metrics snapshots that already own them.
func (a *MessageAdmin) assertConsumerGroupIdle(ctx context.Context, streamId int64, consumerGroupId int64, consumerName string) error {
	// a running consumer heartbeats its worker instances -- any live
	// instance on a group-owned worker means someone is consuming
	workers, err := a.metricController.WorkerSnapshots(ctx)
	if err != nil {
		return err
	}
	for _, snapshot := range workers {
		if snapshot.Owner.ConsumerGroupId == consumerGroupId && snapshot.LiveInstances > 0 {
			return consume.ErrConsumerGroupLive.With("group", consumerName, "group_id", consumerGroupId)
		}
	}

	// every delivery row is a failure that needs a retry or a dead-letter record
	consumerGroup, err := a.metricController.ConsumerGroupSnapshot(ctx, streamId, consumerGroupId, consumerName)
	if err != nil {
		return err
	}
	exceptions := consumerGroup.Exceptions
	total := exceptions.Ready + exceptions.Inflight + exceptions.Deferred + exceptions.Dead
	if total > 0 {
		return consume.ErrConsumerGroupDeliveriesPending.With("group", consumerName, "stream_id", streamId, "group_id", consumerGroupId)
	}
	return nil
}
