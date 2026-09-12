package sqlstreams

import (
	"context"
)

// StreamHandle is the stream's name plus the client, holding no row. Message
// is the payload type every handle under it reads and writes.
// Every verb resolves the name when called; Get is
// the comma-ok read, every other verb returns the not-found error itself.
type StreamHandle[Message Versioned] struct {
	name   string
	client *Client
}

// Streams returns every registered stream, ordered by name.
func (c *Client) Streams(ctx context.Context) ([]*Stream, error) {
	return c.admin.ListStreams(ctx)
}

// Stream names a stream on the client under the payload type Message. No I/O
// and no failure -- each verb on the handle resolves the name when called.
// A caller with no payload type in scope passes RawPayload.
func (c *Client) Stream[Message Versioned](name string) *StreamHandle[Message] {
	return &StreamHandle[Message]{name: name, client: c}
}

// Register declares the named stream, creating its tables on first
// registration. Idempotent; cfg may be nil or sparse.
func (t *StreamHandle[Message]) Register(ctx context.Context, cfg *StreamConfig) (*Stream, error) {
	return t.client.admin.RegisterStream(ctx, t.name, cfg)
}

// Get reads the stream's row. Returns (nil, nil) when the stream is not
// registered.
func (t *StreamHandle[Message]) Get(ctx context.Context) (*Stream, error) {
	return t.client.admin.GetStream(ctx, t.name)
}

// Migrate moves the stream's tables to targetVersion.
func (t *StreamHandle[Message]) Migrate(ctx context.Context, targetVersion int64) error {
	return t.client.admin.MigrateStream(ctx, t.name, targetVersion)
}

// MigrationVersion reads the version the stream's tables are at. Returns
// ErrStreamNotFound when the stream is not registered.
func (t *StreamHandle[Message]) MigrationVersion(ctx context.Context) (int64, error) {
	return t.client.admin.StreamMigrationVersion(ctx, t.name)
}

// Rename changes the registered name and returns the updated stream. This handle
// keeps its old name; registered instances keep working through the stream id.
// Returns ErrStreamNotFound for an absent stream or ErrStreamNameTaken on conflict.
func (t *StreamHandle[Message]) Rename(ctx context.Context, newName string) (*Stream, error) {
	return t.client.admin.RenameStream(ctx, t.name, newName)
}

// Destroy permanently deletes the stream, its messages, and every consumer
// group on it. Refused unless ClientConfig.AllowDestroy is set.
func (t *StreamHandle[Message]) Destroy(ctx context.Context, options *DestroyOptions) error {
	return t.client.admin.DestroyStream(ctx, t.name, options)
}

// Health reports each payload version's retirement verdict, read live from
// the stream's log, compaction heads, and consumer group cursors. An
// unregistered stream returns ErrStreamNotFound.
func (t *StreamHandle[Message]) Health(ctx context.Context) ([]*StreamVersionHealth, error) {
	return t.client.admin.StreamHealth(ctx, t.name)
}

// CompactionHeads returns every key's current compaction head on the
// stream, ordered by message key.
func (t *StreamHandle[Message]) CompactionHeads(ctx context.Context) ([]*StoredMessage[Message], error) {
	return t.client.admin.ListCompactionHeads[Message](ctx, t.name)
}
