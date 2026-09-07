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

// ListMeasurementMessagesByCreatedAt returns all retained series messages in the
// inclusive window, newest first. Returns migrate.ErrNotRegistered before setup.
func (c *MetricsController) ListMeasurementMessagesByCreatedAt(ctx context.Context, messageKey string, start time.Time, end time.Time) ([]*common.StoredMessage[metrics.Measurement], error) {
	if messageKey == "" {
		return nil, errors.New("messageKey must not be empty")
	}
	if start.IsZero() {
		return nil, errors.New("start must not be zero")
	}
	if end.IsZero() {
		return nil, errors.New("end must not be zero")
	}
	if end.Before(start) {
		return nil, fmt.Errorf("end must be >= start %v, got %v", start, end)
	}

	found, err := c.topics.Get(ctx, metrics.MetricsTopicName)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, migrate.ErrNotRegistered.With("topic", metrics.MetricsTopicName)
	}
	return c.heads.ListKeyMessagesByCreatedAt[metrics.Measurement](ctx, found.Id, messageKey, start, end)
}
