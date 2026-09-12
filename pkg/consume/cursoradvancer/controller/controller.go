package controller

import (
	"errors"

	"github.com/allegedlyreliable/sqlstreams/pkg/common/logging"
	"github.com/allegedlyreliable/sqlstreams/pkg/consume/cursoradvancer/controller/datastore"
	iDatastore "github.com/allegedlyreliable/sqlstreams/pkg/datastore"
)

// CursorAdvancerController is the cursor advancer kind's only path to
// persistence: the instance advances committed through it.
type CursorAdvancerController struct {
	Logger logging.Logger

	datastore *datastore.CursorAdvancerDatastore
}

func NewCursorAdvancerController(ds *iDatastore.PostgresDatastore, logger logging.Logger) (*CursorAdvancerController, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	cursorAdvancerDatastore, err := datastore.NewCursorAdvancerDatastore(ds, logger)
	if err != nil {
		return nil, err
	}

	return &CursorAdvancerController{
		Logger:    logger,
		datastore: cursorAdvancerDatastore,
	}, nil
}
