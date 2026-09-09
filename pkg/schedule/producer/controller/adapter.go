package controller

import (
	"github.com/agentstax/sqlstreams/pkg/schedule/producer/controller/datastore"
)

func toDueSchedule(data *datastore.DueScheduleRow) *DueSchedule {
	return &DueSchedule{
		Id:              data.Id,
		Name:            data.Name,
		Expression:      data.Expression,
		StreamName:      data.StreamName,
		Concurrency:     data.Concurrency,
		Timeout:         data.Timeout,
		Payload:         data.Payload,
		SchemaVersion:   data.SchemaVersion,
		Metadata:        data.Metadata,
		NextScheduledAt: data.NextScheduledAt,
		DbNow:           data.DbNow,
	}
}
