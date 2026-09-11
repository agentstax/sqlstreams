package record

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestConcurrentHandlerRecordsAreVisibleBeforeClose(t *testing.T) {
	// setup
	dir := t.TempDir()
	writer := newTestHandlerWriter(t, dir, 4)
	failures := make(chan error, 8)
	var pending sync.WaitGroup

	// test
	for caller := range 8 {
		pending.Go(func() {
			for number := range 128 {
				if err := writer.Write(HandlerRecord{MessageId: int64(caller*128 + number + 1), Consumer: "consumer/orders/processor/c-1", Outcome: HandlerOutcomeSuccess}); err != nil {
					failures <- err
					return
				}
			}
		})
	}
	pending.Wait()
	close(failures)

	// verify
	for err := range failures {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(dir, "*.handler.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[int64]bool)
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		lines := bufio.NewScanner(file)
		for lines.Scan() {
			var row HandlerRecord
			if err := json.Unmarshal(lines.Bytes(), &row); err != nil {
				t.Fatal(err)
			}
			if row.MessageId < 1 || row.MessageId > 1024 || seen[row.MessageId] {
				t.Fatalf("Write message id = %d, want one occurrence of each id 1..1024", row.MessageId)
			}
			if row.Consumer != "consumer/orders/processor/c-1" || row.Outcome != HandlerOutcomeSuccess {
				t.Fatalf("Write identity/outcome = %+v, want original consumer and success", row)
			}
			seen[row.MessageId] = true
		}
		if err := lines.Err(); err != nil {
			t.Fatal(err)
		}
	}
	if len(seen) != 1024 {
		t.Fatalf("Write visible rows = %d, want 1024 before Close", len(seen))
	}
}

// ***************
// *** HELPERS ***
// ***************

func newTestHandlerWriter(t testing.TB, dir string, concurrency int) *HandlerWriter {
	t.Helper()
	writer, err := NewHandlerWriter(dir, "consumer", concurrency)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := writer.Close(); err != nil {
			t.Error(err)
		}
	})
	return writer
}

func BenchmarkHandlerRecording(b *testing.B) {
	b.Run("shared", func(b *testing.B) {
		writer, err := NewWriter(b.TempDir(), "consumer", FileKindHandler)
		if err != nil {
			b.Fatal(err)
		}
		defer writer.Close()
		row := HandlerRecord{Consumer: "consumer/orders/processor/c-1", Stream: "orders", Group: "processor", MessageId: 1, Key: "producer-1", Attempt: 1, Outcome: HandlerOutcomeSuccess}
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if err := writer.Write(row); err != nil {
					b.Error(err)
					return
				}
			}
		})
	})
	b.Run("independent", func(b *testing.B) {
		writer, err := NewHandlerWriter(b.TempDir(), "consumer", 4)
		if err != nil {
			b.Fatal(err)
		}
		defer writer.Close()
		row := HandlerRecord{Consumer: "consumer/orders/processor/c-1", Stream: "orders", Group: "processor", MessageId: 1, Key: "producer-1", Attempt: 1, Outcome: HandlerOutcomeSuccess}
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if err := writer.Write(row); err != nil {
					b.Error(err)
					return
				}
			}
		})
	})
}
