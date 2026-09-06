package admin

import (
	"context"
	"errors"

	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/consume"
	"github.com/agentstax/vulkan/pkg/topic"
)

// SystemOwner resolves the system to its owner. Returns ErrNotRegistered
// until RegisterSystem has run.
func (a *MessageAdmin) SystemOwner(ctx context.Context) (*common.Owner, error) {
	sys, err := a.GetSystem(ctx)
	if err != nil {
		return nil, err
	}
	return common.NewSystemOwner(sys.Id)
}

// TopicOwner resolves the topic registered under name to its owner. Returns
// ErrTopicNotFound when it is missing.
func (a *MessageAdmin) TopicOwner(ctx context.Context, name string) (*common.Owner, error) {
	found, err := a.GetTopic(ctx, name)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, topic.ErrTopicNotFound.With("topic", name)
	}
	return common.NewTopicOwner(found.SystemId, found.Id, found.Name)
}

// ConsumerGroupOwner resolves the group registered under consumerName on topicName to
// its owner. Returns ErrTopicNotFound / ErrGroupNotFound when either side
// is missing.
func (a *MessageAdmin) ConsumerGroupOwner(ctx context.Context, topicName string, consumerName string) (*common.Owner, error) {
	if consumerName == "" {
		return nil, errors.New("consumer name is required")
	}

	found, err := a.GetTopic(ctx, topicName)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, topic.ErrTopicNotFound.With("topic", topicName)
	}

	consumerGroup, err := a.consumerController.GetGroup(ctx, found.Id, consumerName)
	if err != nil {
		return nil, err
	}
	if consumerGroup == nil {
		return nil, consume.ErrGroupNotFound.With("group", consumerName, "topic", topicName)
	}
	return common.NewConsumerGroupOwner(found.SystemId, found.Id, consumerGroup.Id, consumerGroup.Name)
}
