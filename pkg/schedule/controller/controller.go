package controller

import (
	"errors"

	"github.com/agentstax/vulkan/pkg/common/logging"
	iDatastore "github.com/agentstax/vulkan/pkg/datastore"
	"github.com/agentstax/vulkan/pkg/schedule/controller/datastore"
)

type ScheduleController struct {
	Logger logging.Logger

	datastore *datastore.ScheduleDatastore
}

func NewScheduleController(ds *iDatastore.PostgresDatastore, logger logging.Logger) (*ScheduleController, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	scheduleDatastore, err := datastore.NewScheduleDatastore(ds, logger)
	if err != nil {
		return nil, err
	}

	return &ScheduleController{
		Logger:    logger,
		datastore: scheduleDatastore,
	}, nil
}
