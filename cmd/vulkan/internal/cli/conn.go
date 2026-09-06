package cli

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/agentstax/vulkan/pkg/common/logging"
	"github.com/agentstax/vulkan/pkg/datastore"
	vulkan "github.com/agentstax/vulkan/pkg/vulkan"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	databaseURLEnv = "VULKAN_ADMIN_DATABASE_URL"
	schemaEnv      = "VULKAN_ADMIN_SCHEMA"
)

type connection struct {
	pool   *pgxpool.Pool
	config *datastore.PostgresDatastoreConfig
	client *vulkan.Client
}

// newConnection resolves the connection (flag then env), dials Postgres, and
// builds the client over the pool.
// pgx owns the DSN, so sslmode, pool_max_conns, connect_timeout, the
// keyword/value form, and the libpq PG* environment variables all work
// without this package parsing any of them.
// Schema, logger, and retry settings resolve once for the client and advanced reads.
// AllowDestroy is set here because this binary IS the privileged admin tool --
// the gate exists for library embedders, not the CLI (ADMIN_CLI.md).
// Library logs go to stderr, never stdout, which carries the command payload.
// level is ERROR for the one-shot commands, whose own ✓/error output is the
// interface, and INFO for `manager run`, whose log stream IS its output.
// Close releases the pool after all command operations stop.
func newConnection(ctx context.Context, databaseURL string, schema string, level slog.Level) (*connection, error) {
	raw := databaseURL
	if raw == "" {
		raw = os.Getenv(databaseURLEnv)
	}
	if raw == "" {
		return nil, failUsage("no database URL -- pass --database-url or set %s", databaseURLEnv)
	}
	if schema == "" {
		schema = os.Getenv(schemaEnv)
	}

	// the schema reaches CREATE SCHEMA and every table qualifier as written, so
	// a bad identifier is a usage error before anything dials
	datastoreConfig := &datastore.PostgresDatastoreConfig{Schema: schema, Logger: logging.NewDefaultLogger(os.Stderr, level)}
	datastoreConfig.WithDefaults()
	if err := datastoreConfig.Validate(); err != nil {
		return nil, failUsage("%s", err.Error())
	}

	// the wrong scheme is the common paste error and deserves a better message
	// than pgx's; a keyword/value DSN (host=... user=...) carries no scheme and
	// goes straight through
	if scheme, _, found := strings.Cut(raw, "://"); found && scheme != "postgres" && scheme != "postgresql" {
		return nil, failUsage("database URL must start with postgres:// or postgresql:// (got %q)", scheme)
	}

	// the DSN is user input, so failing to parse it is a usage error --
	// everything past this point is the database being unreachable
	poolConfig, err := pgxpool.ParseConfig(raw)
	if err != nil {
		return nil, failUsage("could not parse database URL: %v", err)
	}

	// search_path selects nothing: every vulkan statement names its own
	// schema, so a DSN carrying one silently reads the default installation
	if _, ok := poolConfig.ConnConfig.RuntimeParams["search_path"]; ok {
		return nil, failUsage("database URL sets search_path -- pass --schema or set %s instead", schemaEnv)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, failOp("could not connect to database: %v", err)
	}

	client, err := vulkan.NewClient(ctx, pool, &vulkan.ClientConfig{
		Schema:       datastoreConfig.Schema,
		AllowDestroy: true,
		Logger:       datastoreConfig.Logger,
		Retry:        datastoreConfig.Retry,
	})
	if err != nil {
		pool.Close()
		return nil, failOp("could not connect to database: %v", err)
	}
	return &connection{pool: pool, config: datastoreConfig, client: client}, nil
}

func (c *connection) Close() { c.pool.Close() }
