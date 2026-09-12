package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
)

// ListKeyMessages returns messageKey's retained messages, newest
// first. limit is required: an unbounded read spans the whole retention window.
func (c *CompactionController) ListKeyMessages[Message common.Versioned](ctx context.Context, streamId int64, messageKey string, limit int) ([]*common.StoredMessage[Message], error) {
	if streamId <= 0 {
		return nil, fmt.Errorf("streamId must be > 0, got %d", streamId)
	}
	if messageKey == "" {
		return nil, errors.New("messageKey must not be empty")
	}
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be > 0, got %d", limit)
	}

	data, err := c.datastore.ListKeyMessages(ctx, streamId, messageKey, limit)
	if err != nil {
		return nil, err
	}

	return toStoredMessages[Message](data)
}

// ListKeyMessagesByCreatedAt returns retained messages within inclusive time bounds,
// including superseded rows, ordered by created_at then id descending.
func (c *CompactionController) ListKeyMessagesByCreatedAt[Message common.Versioned](ctx context.Context, streamId int64, messageKey string, start time.Time, end time.Time) ([]*common.StoredMessage[Message], error) {
	if streamId <= 0 {
		return nil, fmt.Errorf("streamId must be > 0, got %d", streamId)
	}
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

	data, err := c.datastore.ListKeyMessagesByCreatedAt(ctx, streamId, messageKey, start, end)
	if err != nil {
		return nil, err
	}
	return toStoredMessages[Message](data)
}
