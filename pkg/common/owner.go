package common

import (
	"errors"
	"fmt"
)

// OwnerKind is which resource an Owner names, derived from the ids it holds.
type OwnerKind string

const (
	// OwnerAny lifts an owner-kind guard -- any kind is admitted, like the
	// manager worker every owner declares.
	OwnerAny OwnerKind = ""

	OwnerSystem        OwnerKind = "system"         // SystemId only
	OwnerTopic         OwnerKind = "topic"          // SystemId and TopicId
	OwnerConsumerGroup OwnerKind = "consumer_group" // all three ids
)

func (k OwnerKind) Validate() error {
	switch k {
	case OwnerSystem, OwnerTopic, OwnerConsumerGroup:
		return nil
	default:
		return fmt.Errorf("must be one of %q, %q, %q, got %q", OwnerSystem, OwnerTopic, OwnerConsumerGroup, k)
	}
}

// Owner is which resource owns a row in a polymorphic table (worker,
// schedule, migration_log).
type Owner struct {
	SystemId        int64  `json:"system_id"`
	TopicId         int64  `json:"topic_id"` // 0 for a system owner
	ConsumerGroupId int64  `json:"group_id"` // 0 unless the owner is a consumer group
	Name            string `json:"owner"`    // "system", the topic name, or the group name
}

func NewSystemOwner(systemId int64) (*Owner, error) {
	if systemId <= 0 {
		return nil, fmt.Errorf("systemId must be > 0, got %d", systemId)
	}
	return &Owner{SystemId: systemId, Name: "system"}, nil
}

func NewTopicOwner(systemId int64, topicId int64, name string) (*Owner, error) {
	if systemId <= 0 {
		return nil, fmt.Errorf("systemId must be > 0, got %d", systemId)
	}
	if topicId <= 0 {
		return nil, fmt.Errorf("topicId must be > 0, got %d", topicId)
	}
	if name == "" {
		return nil, errors.New("name is required")
	}

	return &Owner{SystemId: systemId, TopicId: topicId, Name: name}, nil
}

func NewConsumerGroupOwner(systemId int64, topicId int64, consumerGroupId int64, name string) (*Owner, error) {
	if systemId <= 0 {
		return nil, fmt.Errorf("systemId must be > 0, got %d", systemId)
	}
	if topicId <= 0 {
		return nil, fmt.Errorf("topicId must be > 0, got %d", topicId)
	}
	if consumerGroupId <= 0 {
		return nil, fmt.Errorf("consumerGroupId must be > 0, got %d", consumerGroupId)
	}
	if name == "" {
		return nil, errors.New("name is required")
	}

	return &Owner{SystemId: systemId, TopicId: topicId, ConsumerGroupId: consumerGroupId, Name: name}, nil
}

// Kind reads the owner's kind off its ids: the deepest set id wins.
func (o Owner) Kind() OwnerKind {
	switch {
	case o.ConsumerGroupId > 0:
		return OwnerConsumerGroup
	case o.TopicId > 0:
		return OwnerTopic
	default:
		return OwnerSystem
	}
}
