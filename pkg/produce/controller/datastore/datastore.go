package datastore

import (
	"errors"
	"time"

	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/common/logging"
	"github.com/agentstax/vulkan/pkg/datastore"
)

type ProduceDatastore struct {
	Datastore      *datastore.PostgresDatastore
	DatastoreRetry *common.RetryDatastore // default Wrap classification covers everything except Commit -- classified inline at that call site
	Logger         logging.Logger

	createAheadGate    *createAheadGate
	createAheadTimeout time.Duration
}

func NewProduceDatastore(ds *datastore.PostgresDatastore, logger logging.Logger) (*ProduceDatastore, error) {
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

	// trigger create-ahead at 80% or 95% full partition
	createAheadGate, err := newCreateAheadGate([]float64{0.80, 0.95})
	if err != nil {
		return nil, err
	}

	// the full retry schedule plus per-attempt DB work -- so the timeout only
	// cuts what lock_timeout can't bound (head-read lock waits, network hangs)
	createAheadTimeout := datastoreRetry.CalculateTotalDelay() +
		time.Duration(datastoreRetry.MaxRetries)*createAheadAttemptAllowance

	return &ProduceDatastore{
		Datastore:          ds,
		DatastoreRetry:     datastoreRetry,
		Logger:             logger,
		createAheadGate:    createAheadGate,
		createAheadTimeout: createAheadTimeout,
	}, nil
}
