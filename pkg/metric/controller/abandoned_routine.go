package controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/allegedlyreliable/sqlstreams/pkg/metric"
)

// AbandonedRoutineSnapshot pairs the abandoned/cleared events for (streamId,
// group) read directly off __system.metrics's own message log.
func (c *MetricController) AbandonedRoutineSnapshot(ctx context.Context, streamId int64, group string) (*metric.AbandonedRoutineSnapshot, error) {
	if streamId <= 0 {
		return nil, fmt.Errorf("streamId must be > 0, got %d", streamId)
	}
	if group == "" {
		return nil, errors.New("group is required")
	}

	// one read per event type keeps each query's intent obvious instead of
	// one query encoding both via CASE/HAVING
	routingKey := metric.AbandonedRoutineKey(streamId, group)
	abandoned, err := c.datastore.EventTimestamps(ctx, routingKey, metric.EventAbandoned)
	if err != nil {
		return nil, err
	}
	cleared, err := c.datastore.EventTimestamps(ctx, routingKey, metric.EventCleared)
	if err != nil {
		return nil, err
	}

	return toAbandonedRoutineSnapshot(abandoned, cleared), nil
}
