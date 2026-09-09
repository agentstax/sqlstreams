package sqlstreamstest

// Package sqlstreamstest is the fixture for database tests: a datastore or a
// client over a fresh schema in the database SQLSTREAMS_TEST_DATABASE_URL
// names, dropped when the test ends, plus WaitFor and a counting logger.
// Every fixture verb is the constructor it stands for, with t in place of
// ctx and the fixture's own pool in place of the caller's.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/agentstax/sqlstreams/pkg/datastore"
	"github.com/jackc/pgx/v5/pgxpool"
)

const databaseURLVariable = "SQLSTREAMS_TEST_DATABASE_URL"

// poolMaxConns caps the one pool a test binary opens: every package's tests
// share one server, and a package needs only a few connections at once.
const poolMaxConns = 8

// schemaLabelLimit keeps a schema name well inside Postgres's 63-byte
// identifier limit once the pid and counter are appended.
const schemaLabelLimit = 24

var (
	sharedPoolOnce sync.Once
	sharedPool     *pgxpool.Pool
	sharedPoolErr  error
	schemaCount    atomic.Int64
)

// NewDatastore returns a datastore bound to a fresh schema in the database
// SQLSTREAMS_TEST_DATABASE_URL names, dropped when the test ends. The test
// skips when the variable is unset. cfg may be nil or sparse; its Schema is
// always the fixture's own.
func NewDatastore(t testing.TB, cfg *datastore.PostgresDatastoreConfig) *datastore.PostgresDatastore {
	t.Helper()
	pool := testPool(t)
	ctx := t.Context()

	schema := newSchemaName()
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	})

	if cfg == nil {
		cfg = &datastore.PostgresDatastoreConfig{}
	}
	cfg.Schema = schema
	ds, err := datastore.NewPostgresDatastore(ctx, pool, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return ds
}

// testPool opens the binary's one pool on first use. A test binary is one
// package, so the pool is shared by that package's tests and closes with
// the process.
func testPool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv(databaseURLVariable)
	if url == "" {
		t.Skipf("%s is unset -- set it to a disposable PostgreSQL database URL to run database tests", databaseURLVariable)
	}

	sharedPoolOnce.Do(func() {
		config, err := pgxpool.ParseConfig(url)
		if err != nil {
			sharedPoolErr = err
			return
		}
		config.MaxConns = poolMaxConns
		// one name per binary, so a test can find this pool's own backends in
		// pg_stat_activity through current_setting('application_name')
		config.ConnConfig.RuntimeParams["application_name"] = "sqlstreamstest_" + strconv.Itoa(os.Getpid())
		sharedPool, sharedPoolErr = pgxpool.NewWithConfig(context.Background(), config)
	})
	if sharedPoolErr != nil {
		t.Fatal(sharedPoolErr)
	}
	return sharedPool
}

// newSchemaName is test_<package>_<pid>_<n>: the package for a reader of
// pg_namespace, the pid so two binaries against one database never collide.
func newSchemaName() string {
	binary := strings.TrimSuffix(filepath.Base(os.Args[0]), ".test")
	var label strings.Builder
	for _, character := range strings.ToLower(binary) {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') {
			label.WriteRune(character)
		} else {
			label.WriteByte('_')
		}
	}
	name := label.String()
	if len(name) > schemaLabelLimit {
		name = name[:schemaLabelLimit]
	}
	return fmt.Sprintf("test_%s_%d_%d", name, os.Getpid(), schemaCount.Add(1))
}
