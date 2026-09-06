package admin

import (
	"context"
	"errors"

	"github.com/agentstax/vulkan/pkg/consume"
	"github.com/agentstax/vulkan/pkg/topic"
	"github.com/agentstax/vulkan/pkg/worker"
)

// GetConsumer reads the consumer's registration. Returns (nil, nil), not an error, when
// the topic or the group isn't registered.
func (a *MessageAdmin) GetConsumer(ctx context.Context, topicName string, consumerName string) (*consume.Consumer, error) {
	if consumerName == "" {
		return nil, errors.New("consumer name is required")
	}

	found, err := a.GetTopic(ctx, topicName)
	if err != nil || found == nil {
		return nil, err
	}
	return a.consumerController.GetGroup(ctx, found.Id, consumerName)
}

// ListConsumers lists the topic's registered consumers, ordered by name.
// Returns ErrTopicNotFound when the topic isn't registered.
func (a *MessageAdmin) ListConsumers(ctx context.Context, topicName string) ([]*consume.Consumer, error) {
	found, err := a.GetTopic(ctx, topicName)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, topic.ErrTopicNotFound.With("topic", topicName)
	}
	return a.consumerController.ListGroups(ctx, found.Id)
}

// ListConsumerWorkers lists the consumer group's shared worker rows -- its stored config.
// Returns ErrTopicNotFound / ErrConsumerNotFound when either side is missing.
func (a *MessageAdmin) ListConsumerWorkers(ctx context.Context, topicName string, consumerName string) ([]*worker.Worker, error) {
	consumerGroupOwner, err := a.ConsumerGroupOwner(ctx, topicName, consumerName)
	if err != nil {
		return nil, err
	}
	listed, err := a.workerController.ListWorkers(ctx, consumerGroupOwner)
	if err != nil {
		return nil, err
	}

	// ListWorkers walks the whole owner chain -- keep only the group's own rows
	var workers []*worker.Worker
	for _, row := range listed {
		if row.Owner.ConsumerGroupId == consumerGroupOwner.ConsumerGroupId {
			workers = append(workers, row)
		}
	}
	return workers, nil
}

// DestroyConsumer permanently deletes the consumer group registered under
// consumerName on topicName: its cursor, bindings, leases,
// delivery rows, group-owned workers, and group-owned schedules. The
// topic and its messages are untouched. All instances share this registration.
//
// Returns topic.ErrDestroyDisabled unless MessageAdminConfig.AllowDestroy is set,
// and ErrTopicNotFound / ErrConsumerNotFound when either side is missing.
// Unless options.Force is set:
//   - any consumer instance is live     -> ErrConsumerGroupLive
//   - the group still holds delivery rows    -> ErrConsumerGroupDeliveriesPending
func (a *MessageAdmin) DestroyConsumer(ctx context.Context, topicName string, consumerName string, options *DestroyOptions) error {
	if !a.allowDestroy {
		return topic.ErrDestroyDisabled
	}
	if options == nil {
		options = &DestroyOptions{}
	}

	owner, err := a.ConsumerGroupOwner(ctx, topicName, consumerName)
	if err != nil {
		return err
	}

	if !options.Force {
		if err := a.assertConsumerGroupIdle(ctx, owner.TopicId, owner.ConsumerGroupId, owner.Name); err != nil {
			return err
		}
	}

	return a.consumerController.DeleteGroup(ctx, owner.TopicId, owner.ConsumerGroupId, owner.Name)
}

// assertConsumerGroupIdle is DestroyConsumer's guard: nothing is consuming on the
// group and no delivery rows would be discarded. Both facts come from the
// metrics snapshots that already own them.
func (a *MessageAdmin) assertConsumerGroupIdle(ctx context.Context, topicId int64, consumerGroupId int64, consumerName string) error {
	// a running consumer heartbeats its worker instances -- any live
	// instance on a group-owned worker means someone is consuming
	workers, err := a.metricsController.WorkerSnapshots(ctx)
	if err != nil {
		return err
	}
	for _, snapshot := range workers {
		if snapshot.Owner.ConsumerGroupId == consumerGroupId && snapshot.LiveInstances > 0 {
			return consume.ErrConsumerGroupLive.With("group", consumerName, "group_id", consumerGroupId)
		}
	}

	// every delivery row is a failure that needs a retry or a dead-letter record
	consumerGroup, err := a.metricsController.ConsumerGroupSnapshot(ctx, topicId, consumerGroupId, consumerName)
	if err != nil {
		return err
	}
	exceptions := consumerGroup.Exceptions
	total := exceptions.Ready + exceptions.Inflight + exceptions.Deferred + exceptions.Dead
	if total > 0 {
		return consume.ErrConsumerGroupDeliveriesPending.With("group", consumerName, "topic_id", topicId, "group_id", consumerGroupId)
	}
	return nil
}
