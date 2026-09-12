package sqlstreams

import (
	"context"
)

// ConsumerHandle is a consumer group named on its stream, holding no row.
// Get is the comma-ok read; every other verb returns the not-found error
// itself.
type ConsumerHandle[Message Versioned] struct {
	streamName string
	name       string
	client     *Client
}

// Consumers returns every consumer group registered on the stream, ordered by
// name.
func (t *StreamHandle[Message]) Consumers(ctx context.Context) ([]*Consumer, error) {
	return t.client.admin.ListConsumers(ctx, t.name)
}

// Consumer names a consumer group on this stream. No I/O and no failure --
// each verb on the handle resolves both names when called.
func (t *StreamHandle[Message]) Consumer(name string) *ConsumerHandle[Message] {
	return &ConsumerHandle[Message]{streamName: t.name, name: name, client: t.client}
}

// Register resolves the stream and registers the consumer group on it,
// returning an instance that consumes the stream's Message. cfg is the
// group's declaration -- nil or sparse for the defaults, with cfg.Bindings
// the full pattern set (nil = the whole stream).
func (h *ConsumerHandle[Message]) Register(ctx context.Context, cfg *ConsumerConfig) (*ConsumerInstance[Message], error) {
	instance, err := h.client.consumer.Register[Message](ctx, h.name, h.streamName, cfg)
	if err != nil {
		return nil, err
	}
	return newConsumerInstance(instance, h.client.manager, !h.client.disableManager)
}

// Get reads the group's row. Returns (nil, nil) when the stream or the
// group is not registered.
func (h *ConsumerHandle[Message]) Get(ctx context.Context) (*Consumer, error) {
	return h.client.admin.GetConsumer(ctx, h.streamName, h.name)
}

// Workers returns the group's worker rows -- its stored config.
func (h *ConsumerHandle[Message]) Workers(ctx context.Context) ([]*Worker, error) {
	return h.client.admin.ListConsumerWorkers(ctx, h.streamName, h.name)
}

// Destroy permanently deletes the group: its cursor, bindings, leases,
// delivery rows, group-owned workers and schedules. The stream and its
// messages are untouched. Refused unless ClientConfig.AllowDestroy is set.
func (h *ConsumerHandle[Message]) Destroy(ctx context.Context, options *DestroyOptions) error {
	return h.client.admin.DestroyConsumer(ctx, h.streamName, h.name, options)
}
