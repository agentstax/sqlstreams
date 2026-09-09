package controller

import (
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/pkg/metric"
	"github.com/agentstax/sqlstreams/pkg/metric/controller/datastore"
)

func TestToStreamSnapshot(t *testing.T) {
	groups := []metric.ConsumerGroupSnapshot{{ConsumerGroup: "billing"}}
	snapshot := toStreamSnapshot(41, &datastore.StreamSnapshotRow{
		Partitions:                         7,
		Compacted:                          true,
		CompactionRowsWithoutHead:          3,
		OldestCompactionRowWithoutHeadSecs: 1.5,
	}, groups)

	if snapshot.StreamId != 41 {
		t.Fatalf("StreamId = %d, want 41", snapshot.StreamId)
	}
	if snapshot.Partitions != 7 {
		t.Fatalf("Partitions = %d, want 7", snapshot.Partitions)
	}
	if !snapshot.Compacted {
		t.Fatal("Compacted = false, want true")
	}
	if snapshot.CompactionRowsWithoutHead != 3 {
		t.Fatalf("CompactionRowsWithoutHead = %d, want 3", snapshot.CompactionRowsWithoutHead)
	}
	if snapshot.OldestCompactionRowWithoutHeadAge != 1500*time.Millisecond {
		t.Fatalf("OldestCompactionRowWithoutHeadAge = %v, want 1.5s", snapshot.OldestCompactionRowWithoutHeadAge)
	}
	if len(snapshot.Groups) != 1 || snapshot.Groups[0].ConsumerGroup != "billing" {
		t.Fatalf("Groups = %#v, want billing", snapshot.Groups)
	}
}

func TestToConsumerGroupSnapshotIncludesAbandonedRoutines(t *testing.T) {
	abandoned := metric.AbandonedRoutineSnapshot{Outstanding: 2, Total: 4}
	snapshot := toConsumerGroupSnapshot("billing", &datastore.ConsumerGroupSnapshotRow{}, abandoned)

	if snapshot.AbandonedRoutines != abandoned {
		t.Fatalf("AbandonedRoutines = %#v, want %#v", snapshot.AbandonedRoutines, abandoned)
	}
}
