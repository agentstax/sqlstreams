package admin

import (
	"context"

	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/compaction"
	"github.com/agentstax/vulkan/pkg/datastore"
	"github.com/agentstax/vulkan/pkg/topic"
)

// LockCompactionHead resolves and schema-gates topicName through tx, then
// ensures and locks messageKey's compaction-head row until tx resolves. It
// returns nil when the locked row has no head.
func (a *MessageAdmin) LockCompactionHead[Message common.Versioned](ctx context.Context, tx datastore.Tx, topicName string, messageKey string) (*common.StoredMessage[Message], error) {
	found, err := a.topicController.GetInTx(ctx, tx, topicName)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, topic.ErrTopicNotFound.With("topic", topicName)
	}
	if err := a.topicController.AssertSchemaSupportedInTx(ctx, tx, found.SystemId, found.Id); err != nil {
		return nil, err
	}
	return a.heads.LockHead[Message](ctx, tx, found.Id, messageKey)
}

// GetCompactionHead returns messageKey's current compaction head.
// Returns ErrTopicNotFound when the topic isn't registered and
// ErrCompactionHeadNotFound when no compacted message was produced under
// the key.
func (a *MessageAdmin) GetCompactionHead[Message common.Versioned](ctx context.Context, topicName string, messageKey string) (*common.StoredMessage[Message], error) {
	found, err := a.GetTopic(ctx, topicName)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, topic.ErrTopicNotFound.With("topic", topicName)
	}
	head, err := a.heads.GetHead[Message](ctx, found.Id, messageKey)
	if err != nil {
		return nil, err
	}
	if head == nil {
		return nil, compaction.ErrCompactionHeadNotFound.With("topic", topicName, "topic_id", found.Id, "message_key", messageKey)
	}
	return head, nil
}

// ListCompactionHeads returns every key's current compaction head on the
// topic, ordered by message key.
// Returns ErrTopicNotFound when the topic isn't registered.
func (a *MessageAdmin) ListCompactionHeads[Message common.Versioned](ctx context.Context, topicName string) ([]*common.StoredMessage[Message], error) {
	found, err := a.GetTopic(ctx, topicName)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, topic.ErrTopicNotFound.With("topic", topicName)
	}
	return a.heads.ListHeads[Message](ctx, found.Id)
}

// ListKeyMessages returns messageKey's retained messages, newest first.
// Returns ErrTopicNotFound when the topic isn't registered.
func (a *MessageAdmin) ListKeyMessages[Message common.Versioned](ctx context.Context, topicName string, messageKey string, limit int) ([]*common.StoredMessage[Message], error) {
	found, err := a.GetTopic(ctx, topicName)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, topic.ErrTopicNotFound.With("topic", topicName)
	}
	return a.heads.ListKeyMessages[Message](ctx, found.Id, messageKey, limit)
}
