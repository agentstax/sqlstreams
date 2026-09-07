package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/metrics"
	"github.com/agentstax/vulkan/pkg/migrate"
)

// GetMeasurement returns the retained series head, or nil when absent.
// Returns migrate.ErrNotRegistered until the metrics topic exists.
func (c *MetricsController) GetMeasurement(ctx context.Context, messageKey string) (*common.StoredMessage[metrics.Measurement], error) {
	found, err := c.topics.Get(ctx, metrics.MetricsTopicName)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, migrate.ErrNotRegistered.With("topic", metrics.MetricsTopicName)
	}
	return c.heads.GetHead[metrics.Measurement](ctx, found.Id, messageKey)
}

// GetMeasurementHistory reads an inclusive window ending at database time.
// Retention must exceed the window; returns migrate.ErrNotRegistered before setup.
func (c *MetricsController) GetMeasurementHistory(ctx context.Context, messageKey string, window time.Duration) (*metrics.MeasurementHistory, error) {
	if messageKey == "" {
		return nil, errors.New("messageKey must not be empty")
	}
	if window <= 0 {
		return nil, fmt.Errorf("window must be > 0, got %v", window)
	}

	found, err := c.topics.Get(ctx, metrics.MetricsTopicName)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, migrate.ErrNotRegistered.With("topic", metrics.MetricsTopicName)
	}
	if found.RetentionTTL > 0 && found.RetentionTTL <= window {
		return nil, fmt.Errorf("retention must be > window %v or disabled, got %v", window, found.RetentionTTL)
	}

	current, err := c.datastore.CurrentTime(ctx)
	if err != nil {
		return nil, err
	}
	messages, err := c.heads.ListKeyMessagesByCreatedAt[metrics.Measurement](ctx, found.Id, messageKey, current.Add(-window), current)
	if err != nil {
		return nil, err
	}
	return toMeasurementHistory(current, messages), nil
}
