package controller

import (
	"uuid"

	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/produce"
	"github.com/agentstax/vulkan/pkg/produce/controller/datastore"
)

func toAppend[Message common.Versioned](idempotencyKey uuid.UUID, payload *Message, options produce.ProduceOptions) *datastore.Append[Message] {
	data := &datastore.Append[Message]{
		IdempotencyKey: idempotencyKey,
		Payload:        payload,
		RoutingKey:     options.RoutingKey,
		MessageKey:     options.MessageKey,
		Options:        options.Message,
		ScheduledAt:    options.ScheduledAt,
	}
	if options.Compaction != nil && options.Compaction.Enable {
		data.Compacted = true
		data.CompactionRank = options.Compaction.Rank
	}
	return data
}

func toAppended[Message common.Versioned](data *datastore.Appended[Message]) *Appended[Message] {
	return &Appended[Message]{
		Message:   data.Message,
		Id:        data.Id,
		Duplicate: data.Duplicate,
	}
}
