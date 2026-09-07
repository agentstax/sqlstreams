package controller

import (
	"context"

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
