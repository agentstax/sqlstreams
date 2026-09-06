package datastore

import (
	"context"
	"errors"

	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/common/logging"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresDatastore struct {
	Pool *pgxpool.Pool

	// Schema is the namespace every vulkan table lives in
	Schema string

	// Logger is the one logger every layer built over this datastore reads,
	// bound with the schema attribute
	Logger logging.Logger

	// Retry is the one retry policy every layer built over this datastore reads
	Retry *common.RetryPolicy
}

// NewPostgresDatastore wraps a pool you built and pings it once, so a wrong
// address or credential fails here instead of at the first query.
// cfg may be nil or sparse; the schema defaults to "vulkan".
func NewPostgresDatastore(ctx context.Context, pool *pgxpool.Pool, cfg *PostgresDatastoreConfig) (*PostgresDatastore, error) {
	if pool == nil {
		return nil, errors.New("pool must not be nil")
	}

	if cfg == nil {
		cfg = &PostgresDatastoreConfig{}
	}
	cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}

	// bound on a local, never written back to cfg.Logger: Args concatenate
	// on merge, so a cfg reused across constructions would repeat the schema
	logger := logging.NewPipelineLogger(cfg.Logger, &logging.PipelineLoggerConfig{Args: []any{"schema", cfg.Schema}})

	return &PostgresDatastore{
		Pool:   pool,
		Schema: cfg.Schema,
		Logger: logger,
		Retry:  cfg.Retry,
	}, nil
}
