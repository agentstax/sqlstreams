package controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/agentstax/vulkan/pkg/common"
	iDatastore "github.com/agentstax/vulkan/pkg/datastore"
)

// LockHead ensures messageKey has a compaction-head row, locks that row until
// tx resolves, and returns its current head. A newly created lockable row has
// no head, so this returns nil while still holding the row lock.
func (c *CompactionController) LockHead[Message common.Versioned](ctx context.Context, tx iDatastore.Tx, topicId int64, messageKey string) (*common.StoredMessage[Message], error) {
	if tx == nil {
		return nil, errors.New("tx must not be nil")
	}
	if topicId <= 0 {
		return nil, fmt.Errorf("topicId must be > 0, got %d", topicId)
	}
	if messageKey == "" {
		return nil, errors.New("messageKey must not be empty")
	}

	data, err := c.datastore.LockHead(ctx, tx, topicId, messageKey)
	if err != nil || data == nil {
		return nil, err
	}
	return toStoredMessage[Message](data)
}

// GetHead returns the current compaction head under messageKey,
// or nil if nothing has been published under it.
func (c *CompactionController) GetHead[Message common.Versioned](ctx context.Context, topicId int64, messageKey string) (*common.StoredMessage[Message], error) {
	if topicId <= 0 {
		return nil, fmt.Errorf("topicId must be > 0, got %d", topicId)
	}
	if messageKey == "" {
		return nil, errors.New("messageKey must not be empty")
	}

	data, err := c.datastore.GetHead(ctx, topicId, messageKey)
	if err != nil || data == nil {
		return nil, err
	}
	return toStoredMessage[Message](data)
}

// ListHeads returns every key's current head on the topic, ordered
// by message key.
func (c *CompactionController) ListHeads[Message common.Versioned](ctx context.Context, topicId int64) ([]*common.StoredMessage[Message], error) {
	if topicId <= 0 {
		return nil, fmt.Errorf("topicId must be > 0, got %d", topicId)
	}

	data, err := c.datastore.ListHeads(ctx, topicId)
	if err != nil {
		return nil, err
	}

	heads := make([]*common.StoredMessage[Message], 0, len(data))
	for i := range data {
		head, err := toStoredMessage[Message](&data[i])
		if err != nil {
			return nil, err
		}
		heads = append(heads, head)
	}
	return heads, nil
}
