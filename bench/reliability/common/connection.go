package common

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"

	vulkan "github.com/agentstax/vulkan/pkg/vulkan"
)

// Connection is the pool and client a role runs on, built from the
// POSTGRES_* environment the compose file sets.
type Connection struct {
	Pool   *pgxpool.Pool
	Client *vulkan.Client
}

func NewConnection(ctx context.Context) (*Connection, error) {
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

	pool, err := vulkan.NewPostgresPool(ctx,
		os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD"), host, os.Getenv("POSTGRES_DB"),
		&vulkan.PostgresConnectionConfig{Port: port})
	if err != nil {
		return nil, err
	}
	client, err := vulkan.NewClient(ctx, pool, nil)
	if err != nil {
		pool.Close()
		return nil, err
	}
	return &Connection{Pool: pool, Client: client}, nil
}

func (c *Connection) Close() {
	c.Pool.Close()
}
