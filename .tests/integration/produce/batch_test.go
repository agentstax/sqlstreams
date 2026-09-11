package produce

import (
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/agentstax/sqlstreams/pkg/produce/controller/datastore"
	"github.com/agentstax/sqlstreams/pkg/stream"
)

// batchAttemptTimeout bounds one batch attempt; nothing here waits on it.
const batchAttemptTimeout = 10 * time.Second

// Invariant: a batch commits every message in one transaction, with ids
// ascending in pipeline order; a rerun of the same batch after an ambiguous
// commit reports every item duplicate and stores nothing.
func TestAppendMessageBatchLandsInOrderAndARerunIsAllDuplicates(t *testing.T) {
	// setup
	produces, orders := newProduceDatastore(t)
	ctx := t.Context()
	messages := produces.Datastore.Schema + "." + stream.MessageLogTable(orders.Id)
	appends := []*datastore.Append[produceTestMessage]{plainAppend(uuid.New()), plainAppend(uuid.New()), plainAppend(uuid.New())}

	// test
	appended, failedIndex, err := produces.AppendMessageBatch(ctx, orders.Id, partitionSize, batchAttemptTimeout, appends)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if failedIndex != -1 {
		t.Errorf("AppendMessageBatch() failedIndex = %d, want -1", failedIndex)
	}
	if len(appended) != 3 {
		t.Fatalf("AppendMessageBatch() = %+v, want three results", appended)
	}
	for i := 1; i < len(appended); i++ {
		if appended[i].Id <= appended[i-1].Id {
			t.Errorf("AppendMessageBatch() ids = %d then %d, want ascending in pipeline order", appended[i-1].Id, appended[i].Id)
		}
	}
	for i, data := range appended {
		if data.Duplicate || data.Id == 0 || data.Message == nil {
			t.Errorf("AppendMessageBatch() item %d = %+v, want a new id, Duplicate false, and the payload", i, data)
		}
	}

	// test
	rerun, failedIndex, err := produces.AppendMessageBatch(ctx, orders.Id, partitionSize, batchAttemptTimeout, appends)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if failedIndex != -1 {
		t.Errorf("AppendMessageBatch(rerun) failedIndex = %d, want -1", failedIndex)
	}
	for i, data := range rerun {
		if !data.Duplicate || data.Id != 0 {
			t.Errorf("AppendMessageBatch(rerun) item %d = %+v, want Duplicate true and Id 0", i, data)
		}
	}
	if count := countRows(t, produces, messages); count != 3 {
		t.Errorf("message_log rows after the rerun = %d, want 3", count)
	}
}

// Invariant: a batch whose first attempt finds no partition is rerun after
// the heal with its claims rolled back, so every message lands and none is
// reported duplicate.
func TestAppendMessageBatchRerunAfterAHealLandsEveryMessage(t *testing.T) {
	// setup
	produces, orders := newProduceDatastore(t)
	ctx := t.Context()
	claims := produces.Datastore.Schema + "." + stream.IdempotencyKeyTable(orders.Id)
	advanceSequence(t, produces, orders, 2*partitionSize-1)
	appends := []*datastore.Append[produceTestMessage]{plainAppend(uuid.New()), plainAppend(uuid.New()), plainAppend(uuid.New())}

	// test
	appended, failedIndex, err := produces.AppendMessageBatch(ctx, orders.Id, partitionSize, batchAttemptTimeout, appends)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if failedIndex != -1 {
		t.Errorf("AppendMessageBatch() failedIndex = %d, want -1", failedIndex)
	}
	var landed []int64
	for i, data := range appended {
		if data.Duplicate || data.Id/partitionSize != 2 {
			t.Errorf("AppendMessageBatch() item %d = %+v, want Duplicate false and an id in partition 2", i, data)
		}
		landed = append(landed, data.Id)
	}
	if stored := listMessageIds(t, produces, orders); !slices.Equal(stored, landed) {
		t.Errorf("message ids after the healed batch = %v, want the returned ids %v", stored, landed)
	}
	if count := countRows(t, produces, claims); count != 3 {
		t.Errorf("idempotency_key rows after the healed batch = %d, want 3", count)
	}
	if !partitionExists(t, produces, orders, 2) {
		t.Error("partition 2 after the healed batch = missing, want created")
	}
}
