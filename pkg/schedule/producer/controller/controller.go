package controller

import (
	"errors"

	"github.com/allegedlyreliable/sqlstreams/pkg/common/logging"
	iDatastore "github.com/allegedlyreliable/sqlstreams/pkg/datastore"
	"github.com/allegedlyreliable/sqlstreams/pkg/schedule/producer/controller/datastore"
)

// ScheduleProducerController is the schedule producer kind's only path to
// persistence: the instance scans, claims, and advances schedule rows
// through it.
type ScheduleProducerController struct {
	Logger logging.Logger

	datastore *datastore.ScheduleProducerDatastore
}

func NewScheduleProducerController(ds *iDatastore.PostgresDatastore, logger logging.Logger) (*ScheduleProducerController, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	schedulerDatastore, err := datastore.NewScheduleProducerDatastore(ds, logger)
	if err != nil {
		return nil, err
	}

	return &ScheduleProducerController{
		Logger:    logger,
		datastore: schedulerDatastore,
	}, nil
}
