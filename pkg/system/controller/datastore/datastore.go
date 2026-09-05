package datastore

import (
	"errors"

	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/common/logging"
	"github.com/agentstax/vulkan/pkg/datastore"
)

// SystemDatastore owns the shared control-plane tables.
// Tables:
// - system_config
// - topic_config
// - topic_config_log
// - consumer_group_config
// - worker_config
// - worker_config_log
// - worker_instance
// - schedule_config
// - schedule_cursor
// - migration_log
type SystemDatastore struct {
	Datastore      *datastore.PostgresDatastore
	DatastoreRetry *common.RetryDatastore
	Logger         logging.Logger
}

func NewSystemDatastore(ds *datastore.PostgresDatastore, logger logging.Logger) (*SystemDatastore, error) {
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

	return &SystemDatastore{
		Datastore:      ds,
		DatastoreRetry: datastoreRetry,
		Logger:         logger,
	}, nil
}
