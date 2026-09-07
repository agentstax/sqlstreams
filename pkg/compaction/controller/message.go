package controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/agentstax/vulkan/pkg/common"
)

// ListKeyMessages returns messageKey's retained messages, newest
// first. limit is required: an unbounded read spans the whole retention window.
func (c *CompactionController) ListKeyMessages[Message common.Versioned](ctx context.Context, topicId int64, messageKey string, limit int) ([]*common.StoredMessage[Message], error) {
	if topicId <= 0 {
		return nil, fmt.Errorf("topicId must be > 0, got %d", topicId)
	}
	if messageKey == "" {
		return nil, errors.New("messageKey must not be empty")
	}
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be > 0, got %d", limit)
	}

	data, err := c.datastore.ListKeyMessages(ctx, topicId, messageKey, limit)
	if err != nil {
		return nil, err
	}

	return toStoredMessages[Message](data)
}

// ListKeyMessagesByRank returns retained compacted messages within inclusive
// rank bounds, including superseded rows, ordered by rank then id descending.
func (c *CompactionController) ListKeyMessagesByRank[Message common.Versioned](ctx context.Context, topicId int64, messageKey string, minimumRank int64, maximumRank int64) ([]*common.StoredMessage[Message], error) {
	if topicId <= 0 {
		return nil, fmt.Errorf("topicId must be > 0, got %d", topicId)
	}
	if messageKey == "" {
		return nil, errors.New("messageKey must not be empty")
	}
	if maximumRank < minimumRank {
		return nil, fmt.Errorf("maximumRank must be >= minimumRank %d, got %d", minimumRank, maximumRank)
	}

	data, err := c.datastore.ListKeyMessagesByRank(ctx, topicId, messageKey, minimumRank, maximumRank)
	if err != nil {
		return nil, err
	}
	return toStoredMessages[Message](data)
}
