package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/metric"
	"github.com/agentstax/sqlstreams/pkg/migrate"
)

// ListMeasurements returns each series' newest retained measurement.
// Returns migrate.ErrNotRegistered before the system metrics stream exists.
func (c *MetricController) ListMeasurements(ctx context.Context) ([]*common.StoredMessage[metric.Measurement], error) {
	found, err := c.streams.Get(ctx, metric.MetricStreamName)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, migrate.ErrNotRegistered.With("stream", metric.MetricStreamName)
	}
	return c.heads.ListHeads[metric.Measurement](ctx, found.Id)
}

// GetMeasurement returns the retained series head, or nil when absent.
// Returns migrate.ErrNotRegistered until the metrics stream exists.
func (c *MetricController) GetMeasurement(ctx context.Context, messageKey string) (*common.StoredMessage[metric.Measurement], error) {
	found, err := c.streams.Get(ctx, metric.MetricStreamName)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, migrate.ErrNotRegistered.With("stream", metric.MetricStreamName)
	}
	return c.heads.GetHead[metric.Measurement](ctx, found.Id, messageKey)
}

// GetMeasurementHistory reads an inclusive window ending at database time.
// Retention must exceed the window; returns migrate.ErrNotRegistered before setup.
func (c *MetricController) GetMeasurementHistory(ctx context.Context, messageKey string, window time.Duration) (*metric.MeasurementHistory, error) {
	if messageKey == "" {
		return nil, errors.New("messageKey must not be empty")
	}
	if window <= 0 {
		return nil, fmt.Errorf("window must be > 0, got %v", window)
	}

	found, err := c.streams.Get(ctx, metric.MetricStreamName)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, migrate.ErrNotRegistered.With("stream", metric.MetricStreamName)
	}
	if found.RetentionTTL > 0 && found.RetentionTTL <= window {
		return nil, fmt.Errorf("retention must be > window %v or disabled, got %v", window, found.RetentionTTL)
	}

	current, err := c.datastore.CurrentTime(ctx)
	if err != nil {
		return nil, err
	}
	messages, err := c.heads.ListKeyMessagesByCreatedAt[metric.Measurement](ctx, found.Id, messageKey, current.Add(-window), current)
	if err != nil {
		return nil, err
	}
	return toMeasurementHistory(current, messages), nil
}
