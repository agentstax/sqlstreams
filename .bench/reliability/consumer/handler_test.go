package consumer

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/agentstax/sqlstreams/.bench/reliability/common"
	"github.com/agentstax/sqlstreams/.bench/reliability/record"
	"github.com/agentstax/sqlstreams/pkg/consume"
)

func TestHandleReportsARecordWriteFailure(t *testing.T) {
	writer, err := record.NewHandlerWriter(filepath.Join(t.TempDir(), "records"), "c", 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	failed := make(chan error, 1)
	handler, err := NewHandler("c/c-1", "t", "g", 0, writer, nil, failed, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx := consume.WithMeta(context.Background(), consume.MessageMeta{Id: 7, Attempts: 1})
	if err := handler.Handle(ctx, &common.Order{Producer: "p", Sequence: 1}); err == nil {
		t.Fatal("Handle returned nil on a closed writer")
	}
	select {
	case err := <-failed:
		if err == nil {
			t.Fatal("failed carried nil")
		}
	default:
		t.Fatal("the write failure was not reported on failed")
	}
}

func TestDisabledRecordingCountsSuccessWithoutWritingMessageRecords(t *testing.T) {
	// setup
	handler := newDisabledHandler(t)
	ctx := consume.WithMeta(t.Context(), consume.MessageMeta{Id: 7, Attempts: 1})

	// test
	err := handler.Handle(ctx, &common.Order{Producer: "producer", Sequence: 1})

	// verify
	if err != nil || handler.progress.Success.Load() != 1 {
		t.Errorf("Handle(recording disabled, closed message writer) = %v, success %d, want nil, 1", err, handler.progress.Success.Load())
	}
	select {
	case err := <-handler.failed:
		t.Errorf("Handle(recording disabled) reported %v, want no recorder error", err)
	default:
	}
}

func newDisabledHandler(t testing.TB) *Handler {
	t.Helper()
	directory := t.TempDir()
	writer, err := record.NewHandlerWriter(directory, "consumer", 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	progressWriter, err := record.NewWriter(directory, "consumer", record.FileKindProgress)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { progressWriter.Close() })
	progress, err := record.NewProgress(progressWriter, "consumer", "orders", "processor")
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandler("consumer", "orders", "processor", 0, writer, progress, make(chan error, 1), &HandlerConfig{DisableMessageRecording: true})
	if err != nil {
		t.Fatal(err)
	}
	return handler
}
