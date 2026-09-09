package controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/agentstax/vulkan/pkg/consume"
	"github.com/agentstax/vulkan/pkg/metric"
)

// ConsumerGroupSnapshot is the current picture for one resolved group on a
// topic, with every section filled.
func (c *MetricController) ConsumerGroupSnapshot(ctx context.Context, topicId int64, consumerGroupId int64, consumerGroupName string) (*metric.ConsumerGroupSnapshot, error) {
	if topicId <= 0 {
		return nil, fmt.Errorf("topicId must be > 0, got %d", topicId)
	}
	if consumerGroupId <= 0 {
		return nil, fmt.Errorf("consumerGroupId must be > 0, got %d", consumerGroupId)
	}
	if consumerGroupName == "" {
		return nil, errors.New("consumerGroupName is required")
	}

	data, err := c.datastore.ConsumerGroupSnapshot(ctx, topicId, consumerGroupId)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, consume.ErrConsumerNotFound.With("group", consumerGroupName, "topic_id", topicId, "group_id", consumerGroupId)
	}
	abandonedRoutines, err := c.AbandonedRoutineSnapshot(ctx, topicId, consumerGroupName)
	if err != nil {
		return nil, err
	}

	return toConsumerGroupSnapshot(consumerGroupName, data, *abandonedRoutines), nil
}
