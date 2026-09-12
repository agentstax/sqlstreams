package controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/allegedlyreliable/sqlstreams/pkg/consume"
	"github.com/allegedlyreliable/sqlstreams/pkg/metric"
)

// ConsumerGroupSnapshot is the current picture for one resolved group on a
// stream, with every section filled.
func (c *MetricController) ConsumerGroupSnapshot(ctx context.Context, streamId int64, consumerGroupId int64, consumerGroupName string) (*metric.ConsumerGroupSnapshot, error) {
	if streamId <= 0 {
		return nil, fmt.Errorf("streamId must be > 0, got %d", streamId)
	}
	if consumerGroupId <= 0 {
		return nil, fmt.Errorf("consumerGroupId must be > 0, got %d", consumerGroupId)
	}
	if consumerGroupName == "" {
		return nil, errors.New("consumerGroupName is required")
	}

	data, err := c.datastore.ConsumerGroupSnapshot(ctx, streamId, consumerGroupId)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, consume.ErrConsumerNotFound.With("group", consumerGroupName, "stream_id", streamId, "group_id", consumerGroupId)
	}
	abandonedRoutines, err := c.AbandonedRoutineSnapshot(ctx, streamId, consumerGroupName)
	if err != nil {
		return nil, err
	}

	return toConsumerGroupSnapshot(consumerGroupName, data, *abandonedRoutines), nil
}
