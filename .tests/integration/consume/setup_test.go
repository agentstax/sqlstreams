package consume

import (
	"context"
	"testing"

	"github.com/agentstax/sqlstreams/.tests/integration/postgres"
	"github.com/agentstax/sqlstreams/pkg/consume/messageconsumer/controller/datastore"
	"github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type claimTestMessage struct {
	Id string `json:"id"`
}

func (claimTestMessage) SchemaVersion() int { return 1 }

// claimTest is one registered stream and consumer group, with the qualified
// table names the tests read and write directly.
type claimTest struct {
	groups   *datastore.MessageConsumerGroupDatastore
	pool     *pgxpool.Pool
	streamId int64
	groupId  int64
	messages string
	cursor   string
}

// newClaimTest registers a system, a stream, and a consumer group through
// the client, so the tables are the registry's own and the group's cursor
// starts before any message.
func newClaimTest(t testing.TB) *claimTest {
	t.Helper()
	ctx := t.Context()
	ds := postgres.Start(t)
	client, err := sqlstreams.NewClient(ctx, ds.Pool, &sqlstreams.ClientConfig{Schema: ds.Schema})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.System().Register(ctx, nil); err != nil {
		t.Fatal(err)
	}
	claims := client.Stream[claimTestMessage]("claims")
	registered, err := claims.Register(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	readers := claims.Consumer("readers")
	if _, err := readers.Register(ctx, nil); err != nil {
		t.Fatal(err)
	}
	group, err := readers.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	groups, err := datastore.NewMessageConsumerGroupDatastore(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return &claimTest{
		groups:   groups,
		pool:     ds.Pool,
		streamId: registered.Id,
		groupId:  group.Id,
		messages: ds.Schema + "." + stream.MessageLogTable(registered.Id),
		cursor:   ds.Schema + "." + stream.ConsumerGroupCursorTable(registered.Id),
	}
}

// holdTransaction opens a transaction that owns a transaction id and stays
// open until the test ends or the caller commits it.
func holdTransaction(t testing.TB, pool *pgxpool.Pool) pgx.Tx {
	t.Helper()
	tx, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	if _, err := tx.Exec(t.Context(), "SELECT pg_current_xact_id()"); err != nil {
		t.Fatal(err)
	}
	return tx
}
