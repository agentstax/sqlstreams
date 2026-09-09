package controller

import (
	"errors"

	"github.com/agentstax/sqlstreams/pkg/common/logging"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	"github.com/agentstax/sqlstreams/pkg/stream/janitor/controller/datastore"
)

// JanitorController is the janitor kind's only path to persistence: the
// instance's sweep pass drops and drains expired storage through it.
type JanitorController struct {
	Logger logging.Logger

	datastore *datastore.JanitorDatastore
}

func NewJanitorController(ds *iDatastore.PostgresDatastore, logger logging.Logger) (*JanitorController, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	janitorDatastore, err := datastore.NewJanitorDatastore(ds, logger)
	if err != nil {
		return nil, err
	}

	return &JanitorController{
		Logger:    logger,
		datastore: janitorDatastore,
	}, nil
}
