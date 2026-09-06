package datastore

import "github.com/agentstax/vulkan/pkg/common"

// OwnerColumns selects the owning resource's ID; ancestors are NULL in SQL.
type OwnerColumns struct {
	SystemId        *int64
	TopicId         *int64
	ConsumerGroupId *int64
}

func NewOwnerColumns(owner common.Owner) *OwnerColumns {
	switch owner.Kind() {
	case common.OwnerConsumerGroup:
		return &OwnerColumns{ConsumerGroupId: &owner.ConsumerGroupId}
	case common.OwnerTopic:
		return &OwnerColumns{TopicId: &owner.TopicId}
	default:
		return &OwnerColumns{SystemId: &owner.SystemId}
	}
}
