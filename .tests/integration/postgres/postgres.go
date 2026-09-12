package postgres

// Package postgres is the integration tests' one Docker seam: a Postgres
// container per test binary, a schema per test. SQLSTREAMS_TEST_DATABASE_URL,
// when set, names a server to use instead of starting a container.

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/allegedlyreliable/sqlstreams/pkg/datastore"
	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

const image = "postgres:18"

var (
	serverOnce sync.Once
	serverURL  string
	serverErr  error
	schemas    atomic.Int64
)

// Start returns a datastore over a fresh schema, dropped when the test
// ends, in the Postgres this test binary owns.
func Start(t testing.TB) *datastore.PostgresDatastore {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), server(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	schema := fmt.Sprintf("test_%d", schemas.Add(1))
	if _, err := pool.Exec(t.Context(), "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	})

	ds, err := datastore.NewPostgresDatastore(t.Context(), pool, &datastore.PostgresDatastoreConfig{Schema: schema})
	if err != nil {
		t.Fatal(err)
	}
	return ds
}

// server starts the binary's container on first use. The container is
// removed by testcontainers' reaper when the test process exits.
func server(t testing.TB) string {
	t.Helper()
	serverOnce.Do(func() {
		if url := os.Getenv("SQLSTREAMS_TEST_DATABASE_URL"); url != "" {
			serverURL = url
			return
		}
		ctx := context.Background()
		container, err := tcpostgres.Run(ctx, image,
			tcpostgres.WithDatabase("sqlstreams"),
			tcpostgres.WithUsername("sqlstreams"),
			tcpostgres.WithPassword("sqlstreams"),
			tcpostgres.BasicWaitStrategies(),
		)
		if err != nil {
			serverErr = err
			return
		}
		serverURL, serverErr = container.ConnectionString(ctx, "sslmode=disable")
	})
	if serverErr != nil {
		t.Fatal(serverErr)
	}
	return serverURL
}
