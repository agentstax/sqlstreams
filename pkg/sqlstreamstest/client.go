package sqlstreamstest

import (
	"testing"

	"github.com/agentstax/sqlstreams/pkg/datastore"
	"github.com/agentstax/sqlstreams/pkg/sqlstreams"
)

// NewClient returns a client over ds's pool and schema with the system
// registered, so every table comes from the migration registry. cfg may be
// nil or sparse; its Schema is always ds's.
func NewClient(t testing.TB, ds *datastore.PostgresDatastore, cfg *sqlstreams.ClientConfig) *sqlstreams.Client {
	t.Helper()
	if ds == nil {
		t.Fatal("ds must not be nil")
	}
	if cfg == nil {
		cfg = &sqlstreams.ClientConfig{}
	}
	cfg.Schema = ds.Schema

	ctx := t.Context()
	client, err := sqlstreams.NewClient(ctx, ds.Pool, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.System().Register(ctx, nil); err != nil {
		t.Fatal(err)
	}
	return client
}
