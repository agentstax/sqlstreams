package admin

import (
	"context"

	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/metric"
	"github.com/agentstax/vulkan/pkg/migrate"
	"github.com/agentstax/vulkan/pkg/topic"
)

// TopicMetrics returns the named topic's live snapshot.
func (a *MessageAdmin) TopicMetrics(ctx context.Context, name string) (*metric.TopicSnapshot, error) {
	found, err := a.GetTopic(ctx, name)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, topic.ErrTopicNotFound.With("topic", name)
	}

	return a.metricController.TopicSnapshot(ctx, found.Id)
}

// ConsumerGroupMetrics returns the named consumer group's live snapshot.
// Returns ErrTopicNotFound / ErrConsumerNotFound when either side is missing.
func (a *MessageAdmin) ConsumerGroupMetrics(ctx context.Context, topicName string, consumerName string) (*metric.ConsumerGroupSnapshot, error) {
	owner, err := a.ConsumerGroupOwner(ctx, topicName, consumerName)
	if err != nil {
		return nil, err
	}
	return a.metricController.ConsumerGroupSnapshot(ctx, owner.TopicId, owner.ConsumerGroupId, owner.Name)
}

// ListMeasurements returns the current head per (name, attributes)
// series on __system.metrics.
// Returns migrate.ErrNotRegistered until RegisterSystem has run.
func (a *MessageAdmin) ListMeasurements(ctx context.Context) ([]*common.StoredMessage[metric.Measurement], error) {
	return a.metricController.ListMeasurements(ctx)
}

// GetMeasurement returns one series' current retained measurement, or nil if
// no retained measurement has its message key.
// Returns migrate.ErrNotRegistered until RegisterSystem has run.
func (a *MessageAdmin) GetMeasurement(ctx context.Context, messageKey string) (*common.StoredMessage[metric.Measurement], error) {
	return a.metricController.GetMeasurement(ctx, messageKey)
}

// ListMeasurementMessages returns one series' retained measurements, newest first.
// messageKey is metric.MeasurementKey(name, attributes); limit is required.
// Returns migrate.ErrNotRegistered until RegisterSystem has run.
func (a *MessageAdmin) ListMeasurementMessages(ctx context.Context, messageKey string, limit int) ([]*common.StoredMessage[metric.Measurement], error) {
	found, err := a.metricTopic(ctx)
	if err != nil {
		return nil, err
	}
	return a.heads.ListKeyMessages[metric.Measurement](ctx, found.Id, messageKey, limit)
}

func (a *MessageAdmin) metricTopic(ctx context.Context) (*topic.Topic, error) {
	found, err := a.topicController.Get(ctx, metric.MetricTopicName)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, migrate.ErrNotRegistered.With("topic", metric.MetricTopicName)
	}
	return found, nil
}
