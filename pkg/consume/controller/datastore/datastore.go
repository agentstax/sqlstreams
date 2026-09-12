package datastore

import (
	"errors"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/common/logging"
	"github.com/allegedlyreliable/sqlstreams/pkg/datastore"
)

type ConsumeDatastore struct {
	Datastore      *datastore.PostgresDatastore
	DatastoreRetry *common.RetryDatastore
	Logger         logging.Logger
}

func NewConsumeDatastore(ds *datastore.PostgresDatastore, logger logging.Logger) (*ConsumeDatastore, error) {
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	datastoreRetry, err := common.NewRetryDatastore(ds.Retry, logger)
	if err != nil {
		return nil, err
	}

	return &ConsumeDatastore{
		Datastore:      ds,
		DatastoreRetry: datastoreRetry,
		Logger:         logger,
	}, nil
}
