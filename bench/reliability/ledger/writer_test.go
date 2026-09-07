package ledger

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriterAppendsOneLinePerFact(t *testing.T) {
	dir := t.TempDir()
	writer, err := NewWriter(dir, "p-1", FileProduce)
	if err != nil {
		t.Fatal(err)
	}

	at := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	facts := []ProduceFact{
		{At: at, Kind: ProduceAttempted, Producer: "p-1", Seq: 1, Key: "p-1-1", ScheduledAt: at},
		{At: at, Kind: ProduceCommitted, Producer: "p-1", Seq: 1, Key: "p-1-1", ScheduledAt: at, MessageId: 118402},
	}
	for _, fact := range facts {
		if err := writer.Write(fact); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	file, err := os.Open(filepath.Join(dir, "p-1.produce.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	lines := bufio.NewScanner(file)
	for i, want := range facts {
		if !lines.Scan() {
			t.Fatalf("line %d is missing", i)
		}
		var got ProduceFact
		if err := json.Unmarshal(lines.Bytes(), &got); err != nil {
			t.Fatalf("line %d: %v", i, err)
		}
		if got != want {
			t.Fatalf("line %d read back as %+v, want %+v", i, got, want)
		}
	}
	if lines.Scan() {
		t.Fatalf("extra line: %s", lines.Text())
	}
	if err := lines.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestWriterReopensForAppend(t *testing.T) {
	dir := t.TempDir()
	for range 2 {
		writer, err := NewWriter(dir, "c-1", FileHandler)
		if err != nil {
			t.Fatal(err)
		}
		if err := writer.Write(HandlerFact{Consumer: "c-1", Outcome: HandlerSuccess}); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
	}

	content, err := os.ReadFile(filepath.Join(dir, "c-1.handler.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := 0
	for _, b := range content {
		if b == '\n' {
			lines++
		}
	}
	if lines != 2 {
		t.Fatalf("a second writer on the same file left %d lines, want 2 (append, not truncate)", lines)
	}
}
