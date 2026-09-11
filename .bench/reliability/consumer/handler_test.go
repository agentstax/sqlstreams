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
	handler, err := NewHandler("c/c-1", "t", "g", 0, writer, failed)
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
