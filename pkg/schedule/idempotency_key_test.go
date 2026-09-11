package schedule

import (
	"bytes"
	"testing"
	"time"
)

// Invariant: idempotent produce per (schedule, scheduled time) -- the key
// is a pure function of both, two schedules due in the same millisecond
// never share one, and a later scheduled time sorts after an earlier one.
func TestIdempotencyKeyIsDeterministicDistinctPerScheduleAndTimeOrdered(t *testing.T) {
	// setup
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	later := at.Add(time.Millisecond)

	// test
	key := IdempotencyKey(at, 7)
	same := IdempotencyKey(at, 7)
	sibling := IdempotencyKey(at, 8)
	next := IdempotencyKey(later, 7)

	// verify
	if key != same {
		t.Errorf("IdempotencyKey(at, 7) twice = %v and %v, want equal", key, same)
	}
	if key == sibling {
		t.Errorf("IdempotencyKey(at, 7) and (at, 8) = %v, want distinct keys in one millisecond", key)
	}
	if bytes.Compare(key[:], next[:]) >= 0 {
		t.Errorf("IdempotencyKey(at, 7) = %v sorts at or after (at + 1ms, 7) = %v, want time order", key, next)
	}
	if key[6]>>4 != 7 || key[8]&0xc0 != 0x80 {
		t.Errorf("IdempotencyKey(at, 7) = %v, want version 7 and the RFC 4122 variant", key)
	}
}
