package controller

import (
	"github.com/allegedlyreliable/sqlstreams/pkg/consume/deliveryconsumer/controller/datastore"
)

func toDelivery(data datastore.ExceptionQueueRow) Delivery {
	return Delivery{
		ConsumerGroupId: data.ConsumerGroupId,
		StreamId:        data.StreamId,
		MessageId:       data.MessageId,
		Payload:         data.Payload,
		Status:          data.Status,
		Attempts:        data.Attempts,
		Options:         data.Options,
	}
}

func toExceptionQueueRow(delivery *Delivery) *datastore.ExceptionQueueRow {
	return &datastore.ExceptionQueueRow{
		ConsumerGroupId: delivery.ConsumerGroupId,
		StreamId:        delivery.StreamId,
		MessageId:       delivery.MessageId,
		Payload:         delivery.Payload,
		Status:          delivery.Status,
		Attempts:        delivery.Attempts,
		Options:         delivery.Options,
	}
}
