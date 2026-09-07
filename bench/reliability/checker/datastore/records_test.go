package datastore

import (
	"strings"
	"testing"
)

func TestLineSourceDecodesEveryLine(t *testing.T) {
	lines := strings.NewReader(`{"at":"2026-09-06T12:00:00Z","kind":"attempted","producer":"p-1","seq":1,"key":"p-1-1","scheduled_at":"2026-09-06T12:00:00Z","message_id":0,"duplicate":false,"code":"","error":""}
{"at":"2026-09-06T12:00:00Z","kind":"committed","producer":"p-1","seq":1,"key":"p-1-1","scheduled_at":"2026-09-06T12:00:00Z","message_id":7,"duplicate":false,"code":"","error":""}
`)
	source := newLineSource(lines, produceLayout.decode)

	var rows [][]any
	for source.Next() {
		row, err := source.Values()
		if err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}
	if err := source.Err(); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("decoded %d rows, want 2", len(rows))
	}
	if got := rows[1][6]; got != int64(7) {
		t.Fatalf("second row message_id = %v, want 7", got)
	}
}

func TestLineSourceStopsOnATornLine(t *testing.T) {
	lines := strings.NewReader(`{"at":"2026-09-06T12:00:00Z","kind":"started","role":"producer","name":"hold","status":"started","detail":""}
{"at":"2026-09-06T12:0`)
	source := newLineSource(lines, phaseLayout.decode)

	if !source.Next() {
		t.Fatal("the first, whole line did not decode")
	}
	if source.Next() {
		t.Fatal("the torn second line decoded")
	}
	if err := source.Err(); err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("Err = %v, want a line 2 decode error", err)
	}
}
