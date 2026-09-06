package vulkan

import (
	"context"
)

// ConsumerHandle is a consumer group named on its topic, holding no row.
// Get is the comma-ok read; every other verb returns the not-found error
// itself.
type ConsumerHandle[Message Versioned] struct {
	topicName string
	name      string
	client    *Client
}

// Consumers returns every consumer group registered on the topic, ordered by
// name.
func (t *TopicHandle[Message]) Consumers(ctx context.Context) ([]*Consumer, error) {
	return t.client.admin.ListConsumers(ctx, t.name)
}

// Consumer names a consumer group on this topic. No I/O and no failure --
// each verb on the handle resolves both names when called.
func (t *TopicHandle[Message]) Consumer(name string) *ConsumerHandle[Message] {
	return &ConsumerHandle[Message]{topicName: t.name, name: name, client: t.client}
}

// Register resolves the topic and registers the consumer group on it,
// returning an instance that consumes the topic's Message. cfg is the
// group's declaration -- nil or sparse for the defaults, with cfg.Bindings
// the full pattern set (nil = the whole topic).
func (h *ConsumerHandle[Message]) Register(ctx context.Context, cfg *ConsumerConfig) (*ConsumerInstance[Message], error) {
	instance, err := h.client.consumer.Register[Message](ctx, h.name, h.topicName, cfg)
	if err != nil {
		return nil, err
	}
	return newConsumerInstance(instance, h.client.manager, !h.client.Config.DisableManager)
}

// Get reads the group's row. Returns (nil, nil) when the topic or the
// group is not registered.
func (h *ConsumerHandle[Message]) Get(ctx context.Context) (*Consumer, error) {
	return h.client.admin.GetConsumer(ctx, h.topicName, h.name)
}

// Workers returns the group's worker rows -- its stored config.
func (h *ConsumerHandle[Message]) Workers(ctx context.Context) ([]*Worker, error) {
	return h.client.admin.ListConsumerWorkers(ctx, h.topicName, h.name)
}

// Destroy permanently deletes the group: its cursor, bindings, leases,
// delivery rows, group-owned workers and schedules. The topic and its
// messages are untouched. Refused unless ClientConfig.AllowDestroy is set.
func (h *ConsumerHandle[Message]) Destroy(ctx context.Context, options *DestroyOptions) error {
	return h.client.admin.DestroyConsumer(ctx, h.topicName, h.name, options)
}
