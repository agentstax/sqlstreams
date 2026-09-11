package produce

import (
	"context"
	"errors"
	"testing"
	"uuid"

	"github.com/agentstax/sqlstreams/.tests/integration/postgres"
	iDatastore "github.com/agentstax/sqlstreams/pkg/datastore"
	"github.com/agentstax/sqlstreams/pkg/produce/controller/datastore"
	"github.com/agentstax/sqlstreams/pkg/stream"
	streamcontroller "github.com/agentstax/sqlstreams/pkg/stream/controller"
	systemcontroller "github.com/agentstax/sqlstreams/pkg/system/controller"
)

// partitionSize is the "orders" stream's partition size: large enough that
// no produce in these tests reaches a create-ahead trigger point.
const partitionSize int64 = 1000

// produceTestMessage is the payload every produced test message carries.
type produceTestMessage struct {
	Kind string `json:"kind"`
}

func (produceTestMessage) SchemaVersion() int { return 1 }

// errProducerFailed is what failingProducer returns.
var errProducerFailed = errors.New("producer failed")

// newProduceDatastore registers a system and the "orders" stream at
// partitionSize, and returns the produce datastore with the stream.
func newProduceDatastore(t testing.TB) (*datastore.ProduceDatastore, *stream.Stream) {
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
	orders, err := streams.Register(t.Context(), system.Id, "orders", &stream.StreamConfig{PartitionSize: partitionSize})
	if err != nil {
		t.Fatal(err)
	}
	produces, err := datastore.NewProduceDatastore(ds, ds.Logger)
	if err != nil {
		t.Fatal(err)
	}
	return produces, orders
}

// produceTestMessageFunc is the ProducerFunc that returns one test message.
func produceTestMessageFunc(ctx context.Context, tx iDatastore.Tx) (*produceTestMessage, error) {
	return &produceTestMessage{Kind: "created"}, nil
}

// failingProducer is the ProducerFunc that fails with errProducerFailed.
func failingProducer(ctx context.Context, tx iDatastore.Tx) (*produceTestMessage, error) {
	return nil, errProducerFailed
}

// plainAppend is a keyless, uncompacted append under the idempotency key,
// with the payload a batch stores.
func plainAppend(key uuid.UUID) *datastore.Append[produceTestMessage] {
	return &datastore.Append[produceTestMessage]{
		IdempotencyKey: key,
		Payload:        &produceTestMessage{Kind: "created"},
	}
}

// compactedAppend is an append compacted under messageKey at rank.
func compactedAppend(messageKey string, rank int64) *datastore.Append[produceTestMessage] {
	return &datastore.Append[produceTestMessage]{
		IdempotencyKey: uuid.New(),
		MessageKey:     messageKey,
		Compacted:      true,
		CompactionRank: rank,
	}
}

// advanceSequence moves the stream's message id sequence so the next id is
// value + 1.
func advanceSequence(t testing.TB, produces *datastore.ProduceDatastore, orders *stream.Stream, value int64) {
	t.Helper()
	sequence := produces.Datastore.Schema + "." + stream.MessageLogIdSequence(orders.Id)
	if _, err := produces.Datastore.Pool.Exec(t.Context(), "SELECT setval($1, $2)", sequence, value); err != nil {
		t.Fatal(err)
	}
}

// partitionExists reports whether the stream's partition n is a table.
func partitionExists(t testing.TB, produces *datastore.ProduceDatastore, orders *stream.Stream, n int64) bool {
	t.Helper()
	partition := produces.Datastore.Schema + "." + stream.MessageLogPartitionTable(orders.Id, n)
	var table *string
	if err := produces.Datastore.Pool.QueryRow(t.Context(), "SELECT to_regclass($1)::text", partition).Scan(&table); err != nil {
		t.Fatal(err)
	}
	return table != nil
}

// countRows counts the rows of a schema-qualified table.
func countRows(t testing.TB, produces *datastore.ProduceDatastore, table string) int {
	t.Helper()
	var count int
	if err := produces.Datastore.Pool.QueryRow(t.Context(), "SELECT count(*) FROM "+table).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

// listMessageIds lists the stream's message ids in id order.
func listMessageIds(t testing.TB, produces *datastore.ProduceDatastore, orders *stream.Stream) []int64 {
	t.Helper()
	messages := produces.Datastore.Schema + "." + stream.MessageLogTable(orders.Id)
	var ids []int64
	if err := produces.Datastore.Pool.QueryRow(t.Context(), "SELECT coalesce(array_agg(id ORDER BY id), '{}') FROM "+messages).Scan(&ids); err != nil {
		t.Fatal(err)
	}
	return ids
}

// readHead reads the compaction head's message id and rank for the key.
func readHead(t testing.TB, produces *datastore.ProduceDatastore, orders *stream.Stream, messageKey string) (int64, int64) {
	t.Helper()
	heads := produces.Datastore.Schema + "." + stream.CompactionHeadTable(orders.Id)
	var messageId int64
	var rank int64
	if err := produces.Datastore.Pool.QueryRow(t.Context(), "SELECT message_id, compaction_rank FROM "+heads+" WHERE compaction_key = $1", messageKey).Scan(&messageId, &rank); err != nil {
		t.Fatal(err)
	}
	return messageId, rank
}

// produceTestMessageV2 is the payload a later message schema carries.
type produceTestMessageV2 struct {
	Kind string `json:"kind"`
}

func (produceTestMessageV2) SchemaVersion() int { return 2 }

// produceTestMessageV2Func is the ProducerFunc that returns one message of
// the later schema.
func produceTestMessageV2Func(ctx context.Context, tx iDatastore.Tx) (*produceTestMessageV2, error) {
	return &produceTestMessageV2{Kind: "created"}, nil
}

// compactedAppendV2 is an append of the later schema compacted under
// messageKey at rank.
func compactedAppendV2(messageKey string, rank int64) *datastore.Append[produceTestMessageV2] {
	return &datastore.Append[produceTestMessageV2]{
		IdempotencyKey: uuid.New(),
		MessageKey:     messageKey,
		Compacted:      true,
		CompactionRank: rank,
	}
}
