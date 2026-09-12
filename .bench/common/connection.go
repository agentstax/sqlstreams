package common

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"

	sqlstreams "github.com/agentstax/sqlstreams/client"
)

// Connection is the pool and client a role runs on, built from the
// POSTGRES_* environment supplied by the caller or Compose.
type Connection struct {
	Pool   *pgxpool.Pool
	Client *sqlstreams.Client
}

func NewConnection(ctx context.Context, maxConns int) (*Connection, error) {
	port := 5432
	if raw := os.Getenv("POSTGRES_PORT"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("POSTGRES_PORT must be an integer, got %q", raw)
		}
		port = parsed
	}
	host := os.Getenv("POSTGRES_HOST")
	if host == "" {
		host = "localhost"
	}

	pool, err := sqlstreams.NewPostgresPool(ctx,
		os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD"), host, os.Getenv("POSTGRES_DB"),
		&sqlstreams.PostgresConnectionConfig{Port: port, MaxConns: maxConns})
	if err != nil {
		return nil, err
	}
	effective := pool.Config()
	fmt.Fprintf(os.Stderr, "effective runtime: pool_min=%d pool_max=%d max_lifetime=%s max_idle=%s health_check=%s query_mode=%s GOMAXPROCS=%d GOGC=%s GOMEMLIMIT=%s\n",
		effective.MinConns, effective.MaxConns, effective.MaxConnLifetime, effective.MaxConnIdleTime, effective.HealthCheckPeriod,
		effective.ConnConfig.DefaultQueryExecMode, runtime.GOMAXPROCS(0), os.Getenv("GOGC"), os.Getenv("GOMEMLIMIT"))
	client, err := sqlstreams.NewClient(ctx, pool, nil)
	if err != nil {
		pool.Close()
		return nil, err
	}
	return &Connection{Pool: pool, Client: client}, nil
}

func (c *Connection) Close() {
	c.Pool.Close()
}
