package controller

import (
	"encoding/json"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/compaction/controller/datastore"
)

func toStoredMessage[Message common.Versioned](data *datastore.MessageLogRow) (*common.StoredMessage[Message], error) {
	var message Message
	if err := json.Unmarshal(data.Payload, &message); err != nil {
		return nil, err
	}
	return &common.StoredMessage[Message]{
		Id:             data.Id,
		Message:        &message,
		CreatedAt:      data.CreatedAt,
		RoutingKey:     data.RoutingKey,
		MessageKey:     data.MessageKey,
		CompactionRank: data.CompactionRank,
	}, nil
}

func toStoredMessages[Message common.Versioned](data []datastore.MessageLogRow) ([]*common.StoredMessage[Message], error) {
	messages := make([]*common.StoredMessage[Message], 0, len(data))
	for _, row := range data {
		message, err := toStoredMessage[Message](&row)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, nil
}
