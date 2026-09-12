package datastore

import "github.com/allegedlyreliable/sqlstreams/pkg/common"

// OwnerColumns selects the owning resource's ID; ancestors are NULL in SQL.
type OwnerColumns struct {
	SystemId        *int64
	StreamId        *int64
	ConsumerGroupId *int64
}

func NewOwnerColumns(owner common.Owner) *OwnerColumns {
	switch owner.Kind() {
	case common.OwnerConsumerGroup:
		return &OwnerColumns{ConsumerGroupId: &owner.ConsumerGroupId}
	case common.OwnerStream:
		return &OwnerColumns{StreamId: &owner.StreamId}
	default:
		return &OwnerColumns{SystemId: &owner.SystemId}
	}
}
