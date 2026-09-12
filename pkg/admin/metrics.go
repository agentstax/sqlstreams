package admin

import (
	"context"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/metric"
	"github.com/allegedlyreliable/sqlstreams/pkg/migrate"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
)

// StreamMetrics returns the named stream's live snapshot.
func (a *MessageAdmin) StreamMetrics(ctx context.Context, name string) (*metric.StreamSnapshot, error) {
	found, err := a.GetStream(ctx, name)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, stream.ErrStreamNotFound.With("stream", name)
	}

	return a.metricController.StreamSnapshot(ctx, found.Id)
}

// ConsumerGroupMetrics returns the named consumer group's live snapshot.
// Returns ErrStreamNotFound / ErrConsumerNotFound when either side is missing.
func (a *MessageAdmin) ConsumerGroupMetrics(ctx context.Context, streamName string, consumerName string) (*metric.ConsumerGroupSnapshot, error) {
	owner, err := a.ConsumerGroupOwner(ctx, streamName, consumerName)
	if err != nil {
		return nil, err
	}
	return a.metricController.ConsumerGroupSnapshot(ctx, owner.StreamId, owner.ConsumerGroupId, owner.Name)
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
	found, err := a.metricStream(ctx)
	if err != nil {
		return nil, err
	}
	return a.heads.ListKeyMessages[metric.Measurement](ctx, found.Id, messageKey, limit)
}

func (a *MessageAdmin) metricStream(ctx context.Context) (*stream.Stream, error) {
	found, err := a.streamController.Get(ctx, metric.MetricStreamName)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, migrate.ErrNotRegistered.With("stream", metric.MetricStreamName)
	}
	return found, nil
}
