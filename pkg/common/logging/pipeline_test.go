package logging

import (
	"context"
	"io"
	"testing"
	"time"
)

// BenchmarkPipelineLoggerOperation prices the debug buffer on the healthy
// path: one operation opens its boundary and narrates four Debug lines the
// WARN sink drops, and no Error drains the ring. The buffer=on column
// against buffer=off is the cost of always-on capture.
func BenchmarkPipelineLoggerOperation(b *testing.B) {
	b.Run("buffer=off", func(b *testing.B) { benchmarkOperation(b, false) })
	b.Run("buffer=on", func(b *testing.B) { benchmarkOperation(b, true) })
}

func benchmarkOperation(b *testing.B, buffer bool) {
	b.Helper()
	sink := NewDefaultLogger(io.Discard)
	logger := NewPipelineLogger(sink, &PipelineLoggerConfig{Args: []any{"stream", "orders", "group", "billing"}, Buffer: buffer, Suppress: true})
	b.ReportAllocs()
	for b.Loop() {
		ctx := WithLogBuffer(context.Background())
		logger.DebugContext(ctx, "message claimed", "message_id", int64(42), "attempt", 1)
		logger.DebugContext(ctx, "handler started", "message_id", int64(42))
		logger.DebugContext(ctx, "handler returned", "message_id", int64(42), "duration", time.Millisecond)
		logger.DebugContext(ctx, "delivery recorded", "message_id", int64(42))
	}
}
