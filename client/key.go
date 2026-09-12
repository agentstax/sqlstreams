package sqlstreams

import (
	"context"
)

// KeyHandle is one message key on its stream plus the client, holding no
// row. Every verb resolves the stream when called and returns the not-found
// error itself.
type KeyHandle[Message Versioned] struct {
	streamName string
	messageKey string
	client     *Client
}

// Key names a message key on this stream. No I/O and no failure -- each verb
// on the handle resolves the stream when called.
func (t *StreamHandle[Message]) Key(messageKey string) *KeyHandle[Message] {
	return &KeyHandle[Message]{streamName: t.name, messageKey: messageKey, client: t.client}
}

// CompactionHead returns the key's current compaction head, or
// ErrCompactionHeadNotFound if no compacted message was produced under it.
func (k *KeyHandle[Message]) CompactionHead(ctx context.Context) (*StoredMessage[Message], error) {
	return k.client.admin.GetCompactionHead[Message](ctx, k.streamName, k.messageKey)
}

// LockCompactionHead ensures and locks this key's compaction-head row until
// tx resolves. It returns nil when the locked row has no head.
func (k *KeyHandle[Message]) LockCompactionHead(ctx context.Context, tx Tx) (*StoredMessage[Message], error) {
	return k.client.admin.LockCompactionHead[Message](ctx, tx, k.streamName, k.messageKey)
}

// Messages returns the key's retained messages, newest first.
func (k *KeyHandle[Message]) Messages(ctx context.Context, limit int) ([]*StoredMessage[Message], error) {
	return k.client.admin.ListKeyMessages[Message](ctx, k.streamName, k.messageKey, limit)
}
