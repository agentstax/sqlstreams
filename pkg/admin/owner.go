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

// GroupOwner resolves the group registered under groupName on topicName to
// its owner. Returns ErrTopicNotFound / ErrGroupNotFound when either side
// is missing.
func (a *MessageAdmin) GroupOwner(ctx context.Context, topicName string, groupName string) (*common.Owner, error) {
	if groupName == "" {
		return nil, errors.New("group name is required")
	}

	found, err := a.GetTopic(ctx, topicName)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, topic.ErrTopicNotFound.With("topic", topicName)
	}

	group, err := a.consumerController.GetGroup(ctx, found.Id, groupName)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, consume.ErrGroupNotFound.With("group", groupName, "topic", topicName)
	}
	return common.NewConsumerGroupOwner(found.SystemId, found.Id, group.Id, group.Name)
}
