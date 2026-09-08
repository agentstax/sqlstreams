package runner

import (
	"context"

	"github.com/agentstax/vulkan/bench/reliability/common"
	"github.com/agentstax/vulkan/bench/reliability/scenario"
	vulkan "github.com/agentstax/vulkan/pkg/vulkan"
)

// The scenario's [input] section, translated into the library's own
// declarations. Every role registers: registration is idempotent and
// newest-wins, so whichever role starts first bootstraps and the rest agree.

// registeredTopic is one declared topic and its handle, in declaration
// order.
type registeredTopic struct {
	declared scenario.TopicDeclaration
	handle   *vulkan.TopicHandle[common.Order]
}

func (r *Runner) registerTopics(ctx context.Context) ([]registeredTopic, error) {
	if err := r.connection.Client.System().Register(ctx, nil); err != nil {
		return nil, err
	}
	registered := make([]registeredTopic, 0, len(r.declared.Topics))
	for _, declared := range r.declared.Topics {
		handle := r.connection.Client.Topic[common.Order](declared.Name)
		if _, err := handle.Register(ctx, &vulkan.TopicConfig{DeliveryLogMode: declared.DeliveryLogMode}); err != nil {
			return nil, err
		}
		registered = append(registered, registeredTopic{declared: declared, handle: handle})
	}
	return registered, nil
}

// producerConfig is the producer line; a zero batch concurrency is the
// library's own default.
func producerConfig(declared *scenario.Scenario) *vulkan.ProducerConfig {
	cfg := &vulkan.ProducerConfig{}
	cfg.Batch.ConcurrencyLimit = declared.ProducerBatchConcurrency
	cfg.Batch.MaxSize = declared.ProducerBatchSize
	return cfg
}

// consumerConfig is the "N retries then dead" half of a consumers line.
func consumerConfig(group scenario.GroupDeclaration) *vulkan.ConsumerConfig {
	return &vulkan.ConsumerConfig{
		Message: &vulkan.MessageOptions{
			Retry: &vulkan.RetryPolicy{MaxRetries: group.MaxRetries},
		},
	}
}

// consumeOptions is the "batch N" half; a zero BatchLimit is the library's
// own default.
func consumeOptions(group scenario.GroupDeclaration) *vulkan.ConsumeOptions {
	return &vulkan.ConsumeOptions{BatchLimit: group.BatchLimit, QueueSize: group.QueueSize, MessageConcurrency: group.MessageConcurrency, ClaimPollRate: group.ClaimPollRate}
}
