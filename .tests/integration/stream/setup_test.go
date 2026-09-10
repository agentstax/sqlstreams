package stream

import (
	"github.com/agentstax/sqlstreams/.tests/integration/postgres"
	"github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"github.com/agentstax/sqlstreams/pkg/stream/janitor/controller/datastore"
	"testing"
)

type retentionTestMessage struct{}

func (retentionTestMessage) SchemaVersion() int { return 1 }

func newRetentionJanitor(t testing.TB) (*datastore.JanitorDatastore, *stream.Stream) {
	t.Helper()
	ds := postgres.Start(t)
	client, err := sqlstreams.NewClient(t.Context(), ds.Pool, &sqlstreams.ClientConfig{Schema: ds.Schema})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.System().Register(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	orders := client.Stream[retentionTestMessage]("orders")
	registered, err := orders.Register(t.Context(), &stream.StreamConfig{PartitionSize: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := orders.Consumer("processor").Register(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	janitor, err := datastore.NewJanitorDatastore(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return janitor, registered
}

func fillRetentionPartitions(t testing.TB, janitor *datastore.JanitorDatastore) {
	t.Helper()
	ds := janitor.Datastore
	client, err := sqlstreams.NewClient(t.Context(), ds.Pool, &sqlstreams.ClientConfig{Schema: ds.Schema})
	if err != nil {
		t.Fatal(err)
	}
	producer, err := client.Stream[retentionTestMessage]("orders").Producer().Register(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for range 1001 {
		if _, err := producer.Produce(t.Context(), &retentionTestMessage{}, nil); err != nil {
			t.Fatal(err)
		}
	}
}

func seedRetentionKeys(t testing.TB, janitor *datastore.JanitorDatastore, keys string) {
	t.Helper()
	if _, err := janitor.Datastore.Pool.Exec(t.Context(), "INSERT INTO "+keys+" (idempotency_key,created_at) VALUES ('00000000-0000-0000-0000-000000000001',now()),('ffffffff-ffff-ffff-ffff-ffffffffffff',now()-interval '3 hours'),('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',now()-interval '2 hours'),('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',now()-interval '2 hours')"); err != nil {
		t.Fatal(err)
	}
}

func seedRetentionMessages(t testing.TB, janitor *datastore.JanitorDatastore, messages string, cursor string) {
	t.Helper()
	if _, err := janitor.Datastore.Pool.Exec(t.Context(), "INSERT INTO "+messages+" (id,schema_version,payload,created_at) SELECT id,1,'{}',CASE WHEN id<=4 THEN now()-interval '2 hours' ELSE now() END FROM generate_series(1,10) id"); err != nil {
		t.Fatal(err)
	}
	if _, err := janitor.Datastore.Pool.Exec(t.Context(), "UPDATE "+cursor+" SET committed=3"); err != nil {
		t.Fatal(err)
	}
}
