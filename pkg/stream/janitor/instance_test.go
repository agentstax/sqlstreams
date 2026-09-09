package janitor

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/common/logging"
	"github.com/agentstax/sqlstreams/pkg/datastore"
	"github.com/agentstax/sqlstreams/pkg/stream"
	janitorcontroller "github.com/agentstax/sqlstreams/pkg/stream/janitor/controller"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Waiting at pool acquisition exercises the complete controller/retry path
// without opening a database connection.
type sweepAcquireTracer struct {
	contexts []context.Context
	cancel   context.CancelFunc
}

func (t *sweepAcquireTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return ctx
}

func (t *sweepAcquireTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
}

func (t *sweepAcquireTracer) TraceAcquireStart(ctx context.Context, pool *pgxpool.Pool, data pgxpool.TraceAcquireStartData) context.Context {
	t.contexts = append(t.contexts, ctx)
	if t.cancel != nil {
		t.cancel()
	}
	<-ctx.Done()
	return ctx
}

func (t *sweepAcquireTracer) TraceAcquireEnd(ctx context.Context, pool *pgxpool.Pool, data pgxpool.TraceAcquireEndData) {
}

func TestSweepStepDeadlinesAndParentCancellation(t *testing.T) {
	for _, cancelParent := range []bool{false, true} {
		name := "step deadlines"
		if cancelParent {
			name = "parent cancellation"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			tracer := &sweepAcquireTracer{}
			if cancelParent {
				tracer.cancel = cancel
			}
			poolConfig, err := pgxpool.ParseConfig("postgres://unused:unused@localhost/unused")
			if err != nil {
				t.Fatal(err)
			}
			poolConfig.ConnConfig.Tracer = tracer
			pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			logger := logging.NewDefaultLogger(io.Discard)
			controller, err := janitorcontroller.NewJanitorController(&datastore.PostgresDatastore{
				Pool: pool, Schema: "sqlstreams", Logger: logger, Retry: common.NewDefaultRetryPolicy(),
			}, logger)
			if err != nil {
				t.Fatal(err)
			}
			instance := &JanitorInstance{
				Stream:     &stream.Stream{Id: 42, PartitionSize: 1000000, RetentionTTL: time.Hour, IdempotencyKeyTTL: time.Hour, EmptyCompactionHeadTTL: time.Hour, DeliveryLogMode: stream.DeliveryLogModeOff},
				Config:     (&JanitorConfig{CleanupTimeout: 10 * time.Millisecond}).WithDefaults(),
				controller: controller,
				metadata:   defaultJanitorMetadata(),
			}

			err = instance.sweep(ctx)
			if cancelParent {
				if !errors.Is(err, context.Canceled) || len(tracer.contexts) != 1 {
					t.Fatalf("parent cancellation: error = %v, acquisitions = %d", err, len(tracer.contexts))
				}
				return
			}
			if !errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
				t.Fatalf("step deadlines: error = %v, parent error = %v", err, ctx.Err())
			}
			joined, ok := err.(interface{ Unwrap() []error })
			if !ok || len(joined.Unwrap()) != 5 || len(tracer.contexts) != 5 {
				t.Fatalf("expected five step errors and acquisitions: error = %v, acquisitions = %d", err, len(tracer.contexts))
			}
			for index, stepCtx := range tracer.contexts {
				deadline, ok := stepCtx.Deadline()
				if !ok {
					t.Fatal("step has no deadline")
				}
				if index > 0 {
					previous, _ := tracer.contexts[index-1].Deadline()
					if !deadline.After(previous) {
						t.Fatal("step reused an earlier deadline")
					}
				}
			}
		})
	}
}
