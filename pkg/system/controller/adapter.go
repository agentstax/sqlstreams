package controller

import (
	"github.com/allegedlyreliable/sqlstreams/pkg/system"
	"github.com/allegedlyreliable/sqlstreams/pkg/system/controller/datastore"
)

func toSystem(data *datastore.SystemConfigRow) *system.System {
	return &system.System{
		Id:        data.Id,
		CreatedAt: data.CreatedAt,
		UpdatedAt: data.UpdatedAt,
	}
}
