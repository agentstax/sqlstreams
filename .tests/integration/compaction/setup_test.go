package compaction

import (
	"context"
	"fmt"
	"testing"
	"time"
	"uuid"

	"github.com/allegedlyreliable/sqlstreams/.tests/integration/postgres"
	"github.com/allegedlyreliable/sqlstreams/pkg/compaction/controller/datastore"
	iDatastore "github.com/allegedlyreliable/sqlstreams/pkg/datastore"
	producedatastore "github.com/allegedlyreliable/sqlstreams/pkg/produce/controller/datastore"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
	streamcontroller "github.com/allegedlyreliable/sqlstreams/pkg/stream/controller"
	systemcontroller "github.com/allegedlyreliable/sqlstreams/pkg/system/controller"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// compactionTestMessage is the payload every produced test message carries.
type compactionTestMessage struct {
	Kind string `json:"kind"`
}

func (compactionTestMessage) SchemaVersion() int { return 1 }

// newCompactionDatastore registers a system and the "orders" stream, and
// returns the compaction datastore with the stream.
func newCompactionDatastore(t testing.TB) (*datastore.CompactionDatastore, *stream.Stream) {
	t.Helper()
	ds := postgres.Start(t)
	systems, err := systemcontroller.NewSystemController(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	system, err := systems.Register(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	streams, err := streamcontroller.NewStreamController(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	orders, err := streams.Register(t.Context(), system.Id, "orders", nil)
	if err != nil {
		t.Fatal(err)
	}
	heads, err := datastore.NewCompactionDatastore(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return heads, orders
}

// newProduceDatastore is the produce datastore over the same schema.
func newProduceDatastore(t testing.TB, heads *datastore.CompactionDatastore) *producedatastore.ProduceDatastore {
	t.Helper()
	produces, err := producedatastore.NewProduceDatastore(heads.Datastore, heads.Datastore.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return produces
}

// produceCompactionTestMessage is the ProducerFunc that returns one test
// message.
func produceCompactionTestMessage(ctx context.Context, tx iDatastore.Tx) (*compactionTestMessage, error) {
	return &compactionTestMessage{Kind: "created"}, nil
}

// compactedAppend is an append compacted under messageKey at rank, routed
// under "orders.updated".
func compactedAppend(messageKey string, rank int64) *producedatastore.Append[compactionTestMessage] {
	return &producedatastore.Append[compactionTestMessage]{
		IdempotencyKey: uuid.New(),
		RoutingKey:     "orders.updated",
		MessageKey:     messageKey,
		Compacted:      true,
		CompactionRank: rank,
	}
}

// produceCompacted produces one message compacted under messageKey at rank
// and returns its id.
func produceCompacted(t testing.TB, heads *datastore.CompactionDatastore, orders *stream.Stream, messageKey string, rank int64) int64 {
	t.Helper()
	produces := newProduceDatastore(t, heads)
	appended, err := produces.AppendMessage(t.Context(), orders.Id, orders.PartitionSize, produceCompactionTestMessage, compactedAppend(messageKey, rank))
	if err != nil {
		t.Fatal(err)
	}
	return appended.Id
}

// produceUncompacted produces one keyless-routed, uncompacted message under
// messageKey and returns its id.
func produceUncompacted(t testing.TB, heads *datastore.CompactionDatastore, orders *stream.Stream, messageKey string) int64 {
	t.Helper()
	produces := newProduceDatastore(t, heads)
	data := &producedatastore.Append[compactionTestMessage]{IdempotencyKey: uuid.New(), MessageKey: messageKey}
	appended, err := produces.AppendMessage(t.Context(), orders.Id, orders.PartitionSize, produceCompactionTestMessage, data)
	if err != nil {
		t.Fatal(err)
	}
	return appended.Id
}

// lockEmptyHead runs LockHead for messageKey in its own committed
// transaction, leaving the key's lockable row with no head.
func lockEmptyHead(t testing.TB, heads *datastore.CompactionDatastore, orders *stream.Stream, messageKey string) {
	t.Helper()
	err := iDatastore.InTransaction(t.Context(), heads.Datastore, func(ctx context.Context, tx iDatastore.Tx) error {
		_, err := heads.LockHead(ctx, tx, orders.Id, messageKey)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// setCreatedAt moves a message's storage time.
func setCreatedAt(t testing.TB, heads *datastore.CompactionDatastore, orders *stream.Stream, messageId int64, at time.Time) {
	t.Helper()
	messages := heads.Datastore.Schema + "." + stream.MessageLogTable(orders.Id)
	if _, err := heads.Datastore.Pool.Exec(t.Context(), "UPDATE "+messages+" SET created_at = $2 WHERE id = $1", messageId, at); err != nil {
		t.Fatal(err)
	}
}

// heldTx is a transaction the test holds open, in the shape LockHead takes.
type heldTx struct {
	pgx.Tx
}

func (h heldTx) Raw() pgx.Tx {
	return h.Tx
}

// holdTransaction opens a transaction the test ends itself; an unended one
// is rolled back at cleanup.
func holdTransaction(t testing.TB, pool *pgxpool.Pool) iDatastore.Tx {
	t.Helper()
	tx, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return heldTx{tx}
}

// setLockTimeout caps how long the transaction waits on a row lock.
func setLockTimeout(t testing.TB, tx iDatastore.Tx, timeout time.Duration) {
	t.Helper()
	if _, err := tx.Exec(t.Context(), fmt.Sprintf("SET LOCAL lock_timeout = %d", timeout.Milliseconds())); err != nil {
		t.Fatal(err)
	}
}
