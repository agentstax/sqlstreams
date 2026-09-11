package datastore

import (
	"testing"
	"time"
)

// Invariant: a message that never ran and is not the head was superseded by
// the next newer message; the head and every message that ran are not.
func TestGroupMessageStatusesPointASupersededMessageAtItsReplacement(t *testing.T) {
	// setup
	group := matchingGroupRow{Id: 1, Name: "processor"}
	third := keyMessageRow{Id: 3, CreatedAt: time.Date(2026, 1, 1, 3, 0, 0, 0, time.UTC)}
	second := keyMessageRow{Id: 2, CreatedAt: time.Date(2026, 1, 1, 2, 0, 0, 0, time.UTC)}
	first := keyMessageRow{Id: 1, CreatedAt: time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)}
	outcomes := map[int64]messageOutcomeRow{1: {Succeeded: true}}

	// test
	statuses := groupMessageStatuses(group, []keyMessageRow{third, second, first}, third.Id, outcomes)

	// verify
	if len(statuses) != 3 {
		t.Fatalf("groupMessageStatuses() = %+v, want three rows", statuses)
	}
	if !statuses[0].Head || statuses[0].SupersededBy != nil {
		t.Errorf("groupMessageStatuses()[0] = %+v, want the head with no superseded pointer", statuses[0])
	}
	if statuses[1].SupersededBy == nil || *statuses[1].SupersededBy != third.Id || statuses[1].SupersededAt == nil || !statuses[1].SupersededAt.Equal(third.CreatedAt) {
		t.Errorf("groupMessageStatuses()[1] = %+v, want superseded by message %d at %v", statuses[1], third.Id, third.CreatedAt)
	}
	if !statuses[2].Succeeded || statuses[2].SupersededBy != nil {
		t.Errorf("groupMessageStatuses()[2] = %+v, want succeeded with no superseded pointer", statuses[2])
	}
}
