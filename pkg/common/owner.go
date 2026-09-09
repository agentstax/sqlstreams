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
	OwnerStream        OwnerKind = "stream"         // SystemId and StreamId
	OwnerConsumerGroup OwnerKind = "consumer_group" // all three ids
)

func (k OwnerKind) Validate() error {
	switch k {
	case OwnerSystem, OwnerStream, OwnerConsumerGroup:
		return nil
	default:
		return fmt.Errorf("must be one of %q, %q, %q, got %q", OwnerSystem, OwnerStream, OwnerConsumerGroup, k)
	}
}

// Owner is which resource owns a row in a polymorphic table (worker,
// schedule, migration_log).
type Owner struct {
	SystemId        int64  `json:"system_id"`
	StreamId        int64  `json:"stream_id"` // 0 for a system owner
	ConsumerGroupId int64  `json:"group_id"`  // 0 unless the owner is a consumer group
	Name            string `json:"owner"`     // "system", the stream name, or the group name
}

func NewSystemOwner(systemId int64) (*Owner, error) {
	if systemId <= 0 {
		return nil, fmt.Errorf("systemId must be > 0, got %d", systemId)
	}
	return &Owner{SystemId: systemId, Name: "system"}, nil
}

func NewStreamOwner(systemId int64, streamId int64, name string) (*Owner, error) {
	if systemId <= 0 {
		return nil, fmt.Errorf("systemId must be > 0, got %d", systemId)
	}
	if streamId <= 0 {
		return nil, fmt.Errorf("streamId must be > 0, got %d", streamId)
	}
	if name == "" {
		return nil, errors.New("name is required")
	}

	return &Owner{SystemId: systemId, StreamId: streamId, Name: name}, nil
}

func NewConsumerGroupOwner(systemId int64, streamId int64, consumerGroupId int64, name string) (*Owner, error) {
	if systemId <= 0 {
		return nil, fmt.Errorf("systemId must be > 0, got %d", systemId)
	}
	if streamId <= 0 {
		return nil, fmt.Errorf("streamId must be > 0, got %d", streamId)
	}
	if consumerGroupId <= 0 {
		return nil, fmt.Errorf("consumerGroupId must be > 0, got %d", consumerGroupId)
	}
	if name == "" {
		return nil, errors.New("name is required")
	}

	return &Owner{SystemId: systemId, StreamId: streamId, ConsumerGroupId: consumerGroupId, Name: name}, nil
}

// Kind reads the owner's kind off its ids: the deepest set id wins.
func (o Owner) Kind() OwnerKind {
	switch {
	case o.ConsumerGroupId > 0:
		return OwnerConsumerGroup
	case o.StreamId > 0:
		return OwnerStream
	default:
		return OwnerSystem
	}
}
