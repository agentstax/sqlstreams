package controller

import (
	"github.com/agentstax/sqlstreams/pkg/consume"
	"github.com/agentstax/sqlstreams/pkg/consume/controller/datastore"
)

func toConsumer(data *datastore.ConsumerGroupConfigRow) *consume.Consumer {
	return &consume.Consumer{
		Id:        data.Id,
		StreamId:  data.StreamId,
		Name:      data.Name,
		CreatedAt: data.CreatedAt,
	}
}

func toBinding(data *datastore.BindingConfigLogRow) *consume.Binding {
	return &consume.Binding{
		ConsumerGroupName: data.GroupName,
		StreamName:        data.StreamName,
		Status:            consume.BindingOutcome(data.Status),
		Patterns:          data.Patterns,
		DeclaredBy:        data.DeclaredBy,
		DeclaredAt:        data.DeclaredAt,
		AttemptedAt:       data.AttemptedAt,
	}
}
