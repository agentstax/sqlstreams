package admin

import (
	"context"

	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/metrics"
	"github.com/agentstax/vulkan/pkg/migrate"
	"github.com/agentstax/vulkan/pkg/topic"
)

// TopicMetrics returns the named topic's live snapshot.
func (a *MessageAdmin) TopicMetrics(ctx context.Context, name string) (*metrics.TopicSnapshot, error) {
	found, err := a.GetTopic(ctx, name)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, topic.ErrTopicNotFound.With("topic", name)
	}

	return a.metricsController.TopicSnapshot(ctx, found.Id)
}

// GroupMetrics returns the named consumer group's live snapshot.
// Returns ErrTopicNotFound / ErrGroupNotFound when either side is missing.
func (a *MessageAdmin) GroupMetrics(ctx context.Context, topicName string, groupName string) (*metrics.ConsumerGroupSnapshot, error) {
	owner, err := a.GroupOwner(ctx, topicName, groupName)
	if err != nil {
		return nil, err
	}
	return a.metricsController.ConsumerGroupSnapshot(ctx, owner.TopicId, owner.ConsumerGroupId, owner.Name)
}

// ListMeasurements returns the current head per (name, attributes)
// series on __system.metrics.
// Returns migrate.ErrNotRegistered until RegisterSystem has run.
func (a *MessageAdmin) ListMeasurements(ctx context.Context) ([]*common.StoredMessage[metrics.Measurement], error) {
	found, err := a.metricsTopic(ctx)
	if err != nil {
		return nil, err
	}
	return a.heads.ListHeads[metrics.Measurement](ctx, found.Id)
}

// GetMeasurement returns one series' current retained measurement, or nil if
// no retained measurement has its message key.
// Returns migrate.ErrNotRegistered until RegisterSystem has run.
func (a *MessageAdmin) GetMeasurement(ctx context.Context, messageKey string) (*common.StoredMessage[metrics.Measurement], error) {
	found, err := a.metricsTopic(ctx)
	if err != nil {
		return nil, err
	}
	return a.heads.GetHead[metrics.Measurement](ctx, found.Id, messageKey)
}

// ListMeasurementMessages returns one series' retained measurements, newest first.
// messageKey is metrics.MeasurementKey(name, attributes); limit is required.
// Returns migrate.ErrNotRegistered until RegisterSystem has run.
func (a *MessageAdmin) ListMeasurementMessages(ctx context.Context, messageKey string, limit int) ([]*common.StoredMessage[metrics.Measurement], error) {
	found, err := a.metricsTopic(ctx)
	if err != nil {
		return nil, err
	}
	return a.heads.ListKeyMessages[metrics.Measurement](ctx, found.Id, messageKey, limit)
}

func (a *MessageAdmin) metricsTopic(ctx context.Context) (*topic.Topic, error) {
	found, err := a.topicController.Get(ctx, metrics.TopicName)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, migrate.ErrNotRegistered.With("topic", metrics.TopicName)
	}
	return found, nil
}
