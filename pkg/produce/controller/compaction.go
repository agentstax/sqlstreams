package controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/agentstax/vulkan/pkg/common"
	iDatastore "github.com/agentstax/vulkan/pkg/datastore"
)

// GetCompactionHeadInTx ensures and locks the key's compaction-head row against
// the caller's tx, so a following produce on the same key is a race-free
// read-modify-write even when the key has no head yet.
func (c *ProduceController) GetCompactionHeadInTx[Message common.Versioned](ctx context.Context, tx iDatastore.Tx, topicId int64, messageKey string) (*common.StoredMessage[Message], error) {
	if tx == nil {
		return nil, errors.New("tx must not be nil")
	}
	if topicId <= 0 {
		return nil, fmt.Errorf("topicId must be > 0, got %d", topicId)
	}
	if messageKey == "" {
		return nil, errors.New("messageKey must not be empty")
	}

	return c.heads.LockHead[Message](ctx, tx, topicId, messageKey)
}
