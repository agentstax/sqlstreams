package coordinator

import (
	"context"

	"github.com/agentstax/vulkan/bench/reliability/lab"
	vulkan "github.com/agentstax/vulkan/pkg/vulkan"
)

// The scenario's [input] section, translated into the library's own
// declarations. Every role registers: registration is idempotent and
// newest-wins, so whichever role starts first bootstraps and the rest agree.

func (c *Coordinator) registerTopic(ctx context.Context) (*vulkan.TopicHandle[lab.Order], error) {
	if err := c.connection.Client.System().Register(ctx, nil); err != nil {
		return nil, err
	}
	orders := c.connection.Client.Topic[lab.Order](c.declared.Topic)
	if _, err := orders.Register(ctx, c.topicConfig()); err != nil {
		return nil, err
	}
	return orders, nil
}

func (c *Coordinator) topicConfig() *vulkan.TopicConfig {
	return &vulkan.TopicConfig{DeliveryLogMode: c.declared.DeliveryLogMode}
}

// consumerConfig is the "N retries then dead" half of the consumers line.
func (c *Coordinator) consumerConfig() *vulkan.ConsumerConfig {
	return &vulkan.ConsumerConfig{
		Message: &vulkan.MessageOptions{
			Retry: &vulkan.RetryPolicy{MaxRetries: c.declared.MaxRetries},
		},
	}
}
