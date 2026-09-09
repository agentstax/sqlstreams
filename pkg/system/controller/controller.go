package controller

import (
	"errors"
	"fmt"

	"github.com/agentstax/sqlstreams/pkg/common/logging"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	"github.com/agentstax/sqlstreams/pkg/system/controller/datastore"
	"github.com/agentstax/sqlstreams/pkg/worker"
)

type SystemController struct {
	Logger logging.Logger

	declarers []worker.Declarer
	datastore *datastore.SystemDatastore
}

// declarers run on every Register to create the system's worker rows --
// pass them only from a registrar; a controller built for reads needs none.
func NewSystemController(ds *iDatastore.PostgresDatastore, logger logging.Logger, declarers ...worker.Declarer) (*SystemController, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}
	for i, declarer := range declarers {
		if declarer == nil {
			return nil, fmt.Errorf("declarer %d must not be nil", i)
		}
	}

	systemDatastore, err := datastore.NewSystemDatastore(ds, logger)
	if err != nil {
		return nil, err
	}

	return &SystemController{
		Logger:    logger,
		declarers: declarers,
		datastore: systemDatastore,
	}, nil
}
