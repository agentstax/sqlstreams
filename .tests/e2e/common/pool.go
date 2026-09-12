package common

import (
	"context"

	sqlstreams "github.com/agentstax/sqlstreams/client"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool connects to the development database every e2e program runs
// against. cfg may be nil for the defaults.
func NewPool(ctx context.Context, cfg *sqlstreams.PostgresConnectionConfig) (*pgxpool.Pool, error) {
	return sqlstreams.NewPostgresPool(ctx, "example_user", "example_password", "localhost", "example_db", cfg)
}
