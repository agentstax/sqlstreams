package controller

import (
	"context"
	"fmt"

	"github.com/agentstax/vulkan/pkg/metric"
)

func (c *MetricController) TopicSnapshot(ctx context.Context, topicId int64) (*metric.TopicSnapshot, error) {
	if topicId <= 0 {
		return nil, fmt.Errorf("topicId must be > 0, got %d", topicId)
	}

	data, err := c.datastore.TopicSnapshot(ctx, topicId)
	if err != nil {
		return nil, err
	}
	consumerGroups, err := c.datastore.ListConsumerGroups(ctx, topicId)
	if err != nil {
		return nil, err
	}

	groups := make([]metric.ConsumerGroupSnapshot, 0, len(consumerGroups))
	for _, consumerGroup := range consumerGroups {
		group, err := c.ConsumerGroupSnapshot(ctx, topicId, consumerGroup.Id, consumerGroup.Name)
		if err != nil {
			return nil, err
		}
		groups = append(groups, *group)
	}

	return toTopicSnapshot(topicId, data, groups), nil
}

// TopicSchemaVersionSnapshots is every payload version present in the topic's
// log, each with every group's lag against it.
func (c *MetricController) TopicSchemaVersionSnapshots(ctx context.Context, topicId int64) ([]metric.TopicSchemaVersionSnapshot, error) {
	if topicId <= 0 {
		return nil, fmt.Errorf("topicId must be > 0, got %d", topicId)
	}

	counts, err := c.datastore.SchemaVersionCounts(ctx, topicId)
	if err != nil {
		return nil, err
	}

	snapshots := make([]metric.TopicSchemaVersionSnapshot, 0, len(counts))
	for _, count := range counts {
		lags, err := c.datastore.ConsumerGroupSchemaVersionLag(ctx, topicId, count.SchemaVersion)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, toTopicSchemaVersionSnapshot(&count, lags))
	}
	return snapshots, nil
}
