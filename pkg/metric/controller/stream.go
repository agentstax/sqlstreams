package controller

import (
	"context"
	"fmt"

	"github.com/allegedlyreliable/sqlstreams/pkg/metric"
)

func (c *MetricController) StreamSnapshot(ctx context.Context, streamId int64) (*metric.StreamSnapshot, error) {
	if streamId <= 0 {
		return nil, fmt.Errorf("streamId must be > 0, got %d", streamId)
	}

	data, err := c.datastore.StreamSnapshot(ctx, streamId)
	if err != nil {
		return nil, err
	}
	consumerGroups, err := c.datastore.ListConsumerGroups(ctx, streamId)
	if err != nil {
		return nil, err
	}

	groups := make([]metric.ConsumerGroupSnapshot, 0, len(consumerGroups))
	for _, consumerGroup := range consumerGroups {
		group, err := c.ConsumerGroupSnapshot(ctx, streamId, consumerGroup.Id, consumerGroup.Name)
		if err != nil {
			return nil, err
		}
		groups = append(groups, *group)
	}

	return toStreamSnapshot(streamId, data, groups), nil
}

// StreamSchemaVersionSnapshots is every payload version present in the stream's
// log, each with every group's lag against it.
func (c *MetricController) StreamSchemaVersionSnapshots(ctx context.Context, streamId int64) ([]metric.StreamSchemaVersionSnapshot, error) {
	if streamId <= 0 {
		return nil, fmt.Errorf("streamId must be > 0, got %d", streamId)
	}

	counts, err := c.datastore.SchemaVersionCounts(ctx, streamId)
	if err != nil {
		return nil, err
	}

	snapshots := make([]metric.StreamSchemaVersionSnapshot, 0, len(counts))
	for _, count := range counts {
		lags, err := c.datastore.ConsumerGroupSchemaVersionLag(ctx, streamId, count.SchemaVersion)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, toStreamSchemaVersionSnapshot(&count, lags))
	}
	return snapshots, nil
}
