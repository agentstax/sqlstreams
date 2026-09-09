package admin

import (
	"context"

	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/compaction"
	"github.com/agentstax/sqlstreams/pkg/datastore"
	"github.com/agentstax/sqlstreams/pkg/stream"
)

// LockCompactionHead resolves and schema-gates streamName through tx, then
// ensures and locks messageKey's compaction-head row until tx resolves. It
// returns nil when the locked row has no head.
func (a *MessageAdmin) LockCompactionHead[Message common.Versioned](ctx context.Context, tx datastore.Tx, streamName string, messageKey string) (*common.StoredMessage[Message], error) {
	found, err := a.streamController.GetInTx(ctx, tx, streamName)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, stream.ErrStreamNotFound.With("stream", streamName)
	}
	if err := a.streamController.AssertSchemaSupportedInTx(ctx, tx, found.SystemId, found.Id); err != nil {
		return nil, err
	}
	return a.heads.LockHead[Message](ctx, tx, found.Id, messageKey)
}

// GetCompactionHead returns messageKey's current compaction head.
// Returns ErrStreamNotFound when the stream isn't registered and
// ErrCompactionHeadNotFound when no compacted message was produced under
// the key.
func (a *MessageAdmin) GetCompactionHead[Message common.Versioned](ctx context.Context, streamName string, messageKey string) (*common.StoredMessage[Message], error) {
	found, err := a.GetStream(ctx, streamName)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, stream.ErrStreamNotFound.With("stream", streamName)
	}
	head, err := a.heads.GetHead[Message](ctx, found.Id, messageKey)
	if err != nil {
		return nil, err
	}
	if head == nil {
		return nil, compaction.ErrCompactionHeadNotFound.With("stream", streamName, "stream_id", found.Id, "message_key", messageKey)
	}
	return head, nil
}

// ListCompactionHeads returns every key's current compaction head on the
// stream, ordered by message key.
// Returns ErrStreamNotFound when the stream isn't registered.
func (a *MessageAdmin) ListCompactionHeads[Message common.Versioned](ctx context.Context, streamName string) ([]*common.StoredMessage[Message], error) {
	found, err := a.GetStream(ctx, streamName)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, stream.ErrStreamNotFound.With("stream", streamName)
	}
	return a.heads.ListHeads[Message](ctx, found.Id)
}

// ListKeyMessages returns messageKey's retained messages, newest first.
// Returns ErrStreamNotFound when the stream isn't registered.
func (a *MessageAdmin) ListKeyMessages[Message common.Versioned](ctx context.Context, streamName string, messageKey string, limit int) ([]*common.StoredMessage[Message], error) {
	found, err := a.GetStream(ctx, streamName)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, stream.ErrStreamNotFound.With("stream", streamName)
	}
	return a.heads.ListKeyMessages[Message](ctx, found.Id, messageKey, limit)
}
