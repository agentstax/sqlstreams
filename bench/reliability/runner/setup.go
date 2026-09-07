package runner

import (
	"context"

	"github.com/agentstax/vulkan/bench/reliability/common"
	vulkan "github.com/agentstax/vulkan/pkg/vulkan"
)

// The scenario's [input] section, translated into the library's own
// declarations. Every role registers: registration is idempotent and
// newest-wins, so whichever role starts first bootstraps and the rest agree.

func (r *Runner) registerTopic(ctx context.Context) (*vulkan.TopicHandle[common.Order], error) {
	if err := r.connection.Client.System().Register(ctx, nil); err != nil {
		return nil, err
	}
	orders := r.connection.Client.Topic[common.Order](r.declared.Topic)
	if _, err := orders.Register(ctx, r.topicConfig()); err != nil {
		return nil, err
	}
	return orders, nil
}

func (r *Runner) topicConfig() *vulkan.TopicConfig {
	return &vulkan.TopicConfig{DeliveryLogMode: r.declared.DeliveryLogMode}
}

// consumerConfig is the "N retries then dead" half of the consumers line.
func (r *Runner) consumerConfig() *vulkan.ConsumerConfig {
	return &vulkan.ConsumerConfig{
		Message: &vulkan.MessageOptions{
			Retry: &vulkan.RetryPolicy{MaxRetries: r.declared.MaxRetries},
		},
	}
}
