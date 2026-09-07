package controller

import (
	"testing"
	"time"

	"github.com/agentstax/vulkan/pkg/metrics"
	"github.com/agentstax/vulkan/pkg/metrics/controller/datastore"
)

func TestToTopicSnapshot(t *testing.T) {
	groups := []metrics.ConsumerGroupSnapshot{{ConsumerGroup: "billing"}}
	snapshot := toTopicSnapshot(41, &datastore.TopicSnapshotRow{
		Partitions:                         7,
		Compacted:                          true,
		CompactionRowsWithoutHead:          3,
		OldestCompactionRowWithoutHeadSecs: 1.5,
	}, groups)

	if snapshot.TopicId != 41 {
		t.Fatalf("TopicId = %d, want 41", snapshot.TopicId)
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
	abandoned := metrics.AbandonedRoutineSnapshot{Outstanding: 2, Total: 4}
	snapshot := toConsumerGroupSnapshot("billing", &datastore.ConsumerGroupSnapshotRow{}, abandoned)

	if snapshot.AbandonedRoutines != abandoned {
		t.Fatalf("AbandonedRoutines = %#v, want %#v", snapshot.AbandonedRoutines, abandoned)
	}
}
