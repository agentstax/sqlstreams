package controller

import (
	"errors"

	"github.com/agentstax/sqlstreams/pkg/common/logging"
	"github.com/agentstax/sqlstreams/pkg/compaction/controller/datastore"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
)

// CompactionController owns compaction-head reads and the transactional
// ensure-and-lock operation. Producing a compacted message still advances the
// head in the produce domain.
type CompactionController struct {
	Logger logging.Logger

	datastore *datastore.CompactionDatastore
}

func NewCompactionController(ds *iDatastore.PostgresDatastore, logger logging.Logger) (*CompactionController, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	compactionDatastore, err := datastore.NewCompactionDatastore(ds, logger)
	if err != nil {
		return nil, err
	}

	return &CompactionController{
		Logger:    logger,
		datastore: compactionDatastore,
	}, nil
}
