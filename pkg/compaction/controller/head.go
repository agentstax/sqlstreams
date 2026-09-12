package controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	iDatastore "github.com/allegedlyreliable/sqlstreams/pkg/datastore"
)

// LockHead ensures messageKey has a compaction-head row, locks that row until
// tx resolves, and returns its current head. A newly created lockable row has
// no head, so this returns nil while still holding the row lock.
func (c *CompactionController) LockHead[Message common.Versioned](ctx context.Context, tx iDatastore.Tx, streamId int64, messageKey string) (*common.StoredMessage[Message], error) {
	if tx == nil {
		return nil, errors.New("tx must not be nil")
	}
	if streamId <= 0 {
		return nil, fmt.Errorf("streamId must be > 0, got %d", streamId)
	}
	if messageKey == "" {
		return nil, errors.New("messageKey must not be empty")
	}

	data, err := c.datastore.LockHead(ctx, tx, streamId, messageKey)
	if err != nil || data == nil {
		return nil, err
	}
	return toStoredMessage[Message](data)
}

// GetHead returns the current compaction head under messageKey,
// or nil if nothing has been published under it.
func (c *CompactionController) GetHead[Message common.Versioned](ctx context.Context, streamId int64, messageKey string) (*common.StoredMessage[Message], error) {
	if streamId <= 0 {
		return nil, fmt.Errorf("streamId must be > 0, got %d", streamId)
	}
	if messageKey == "" {
		return nil, errors.New("messageKey must not be empty")
	}

	data, err := c.datastore.GetHead(ctx, streamId, messageKey)
	if err != nil || data == nil {
		return nil, err
	}
	return toStoredMessage[Message](data)
}

// ListHeads returns every key's current head on the stream, ordered
// by message key.
func (c *CompactionController) ListHeads[Message common.Versioned](ctx context.Context, streamId int64) ([]*common.StoredMessage[Message], error) {
	if streamId <= 0 {
		return nil, fmt.Errorf("streamId must be > 0, got %d", streamId)
	}

	data, err := c.datastore.ListHeads(ctx, streamId)
	if err != nil {
		return nil, err
	}

	return toStoredMessages[Message](data)
}
