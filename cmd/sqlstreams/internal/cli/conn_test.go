package cli

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/pkg/datastore"
)

func TestConnectionRejectsInvalidSettingsBeforeDial(t *testing.T) {
	t.Setenv(databaseURLEnv, "")
	t.Setenv(schemaEnv, "")
	for _, sample := range []struct {
		name   string
		url    string
		schema string
		want   string
	}{
		{"missing URL", "", "", "no database URL"},
		{"invalid schema", "postgres://localhost/unused", "invalid-schema", "Schema must be"},
		{"wrong scheme", "https://localhost/unused", "", "must start with postgres"},
		{"search path", "postgres://localhost/unused?search_path=other", "", "sets search_path"},
	} {
		t.Run(sample.name, func(t *testing.T) {
			connection, err := newConnection(context.Background(), sample.url, sample.schema, slog.LevelError)
			if connection != nil || err == nil || !strings.Contains(err.Error(), sample.want) {
				t.Fatalf("expected %q, got %v", sample.want, err)
			}
		})
	}
}

func TestConnectionPoolOwnership(t *testing.T) {
	url := os.Getenv("SQLSTREAMS_CLI_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set SQLSTREAMS_CLI_TEST_DATABASE_URL for the connection integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	t.Setenv(databaseURLEnv, "https://invalid.example/ignored")
	t.Setenv(schemaEnv, "from_environment")
	connection, err := newConnection(ctx, url, "from_flag", slog.LevelError)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	advanced, err := datastore.NewPostgresDatastore(ctx, connection.pool, connection.config)
	if err != nil {
		t.Fatal(err)
	}
	if advanced.Schema != "from_flag" {
		t.Fatal("flag schema did not reach the advanced datastore")
	}
	if advanced.Pool != connection.pool || advanced.Retry != connection.config.Retry {
		t.Fatal("advanced datastore did not reuse the owned pool and resolved retry settings")
	}
	connection.Close()
	if err := connection.pool.Ping(ctx); err == nil {
		t.Fatal("connection.Close did not close its pool")
	}

	t.Setenv(databaseURLEnv, url)
	fromEnvironment, err := newConnection(ctx, "", "", slog.LevelError)
	if err != nil {
		t.Fatal(err)
	}
	defer fromEnvironment.Close()
	if fromEnvironment.config.Schema != "from_environment" {
		t.Fatal("schema environment fallback was lost")
	}
}
