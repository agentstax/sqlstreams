package controller

import (
	"errors"

	"github.com/agentstax/vulkan/pkg/common/logging"
	"github.com/agentstax/vulkan/pkg/compaction/controller/datastore"
	iDatastore "github.com/agentstax/vulkan/pkg/datastore"
)

// CompactionController reads compaction heads -- the write side
// (the head upsert and the FOR UPDATE read inside a produce transaction)
// belongs to the producer.
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
