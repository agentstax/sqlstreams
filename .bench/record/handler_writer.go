package record

import (
	"errors"
	"fmt"
)

// HandlerWriter records concurrent invocations in separate files. Each write
// flushes before returning; the checker loads every file by its handler suffix.
type HandlerWriter struct {
	writers   []*Writer
	available chan *Writer
}

// NewHandlerWriter opens concurrency files named <name>-<number>.handler.jsonl.
// Reopening appends, preserving evidence across process restarts.
func NewHandlerWriter(dir string, name string, concurrency int) (*HandlerWriter, error) {
	if name == "" {
		return nil, errors.New("name must not be empty")
	}
	if concurrency < 1 {
		return nil, errors.New("concurrency must be >= 1")
	}
	w := &HandlerWriter{available: make(chan *Writer, concurrency)}
	for number := range concurrency {
		writer, err := NewWriter(dir, fmt.Sprintf("%s-%d", name, number+1), FileKindHandler)
		if err != nil {
			return nil, errors.Join(err, w.Close())
		}
		w.writers = append(w.writers, writer)
		w.available <- writer
	}
	return w, nil
}

// Write borrows an idle file for the complete encode and flush, so another
// handler's file write does not hold a shared lock across filesystem calls.
func (w *HandlerWriter) Write(row HandlerRecord) error {
	writer := <-w.available
	defer func() { w.available <- writer }()
	return writer.Write(row)
}

// Close closes every file. The owner stops its consumer sessions first.
func (w *HandlerWriter) Close() error {
	var result error
	for _, writer := range w.writers {
		result = errors.Join(result, writer.Close())
	}
	return result
}
