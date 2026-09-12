package controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/allegedlyreliable/sqlstreams/pkg/consume"
)

// GetGroup resolves a consumer group by its owning stream and name.
// Returns (nil, nil) if the group is not registered on that stream.
func (c *ConsumeController) GetGroup(ctx context.Context, streamId int64, name string) (*consume.Consumer, error) {
	if streamId <= 0 {
		return nil, fmt.Errorf("streamId must be > 0, got %d", streamId)
	}
	if name == "" {
		return nil, errors.New("name is required")
	}

	data, err := c.datastore.GetGroup(ctx, streamId, name)
	if err != nil || data == nil {
		return nil, err
	}
	return toConsumer(data), nil
}

// ListGroups lists the stream's consumer groups, ordered by name.
func (c *ConsumeController) ListGroups(ctx context.Context, streamId int64) ([]*consume.Consumer, error) {
	if streamId <= 0 {
		return nil, fmt.Errorf("streamId must be > 0, got %d", streamId)
	}

	data, err := c.datastore.ListGroups(ctx, streamId)
	if err != nil {
		return nil, err
	}

	groups := make([]*consume.Consumer, 0, len(data))
	for _, row := range data {
		groups = append(groups, toConsumer(&row))
	}
	return groups, nil
}

// RegisterGroup creates the group and its cursor at start; an existing group
// is returned untouched, its position kept.
func (c *ConsumeController) RegisterGroup(ctx context.Context, streamId int64, name string, start consume.CursorPosition) (*consume.Consumer, error) {
	if streamId <= 0 {
		return nil, fmt.Errorf("streamId must be > 0, got %d", streamId)
	}
	if name == "" {
		return nil, errors.New("name is required")
	}
	if err := start.Kind.Validate(); err != nil {
		return nil, fmt.Errorf("start.Kind: %w", err)
	}

	data, err := c.datastore.RegisterGroup(ctx, streamId, name, start)
	if err != nil {
		return nil, err
	}
	return toConsumer(data), nil
}

// DeleteGroup deletes the group and every row it owns in one transaction.
// A running consumer stops itself: its worker rows vanish with the group,
// so its next heartbeat fails.
func (c *ConsumeController) DeleteGroup(ctx context.Context, streamId int64, groupId int64, name string) error {
	if streamId <= 0 {
		return fmt.Errorf("streamId must be > 0, got %d", streamId)
	}
	if groupId <= 0 {
		return fmt.Errorf("groupId must be > 0, got %d", groupId)
	}
	if name == "" {
		return errors.New("name is required")
	}

	return c.datastore.DeleteGroup(ctx, streamId, groupId, name)
}
