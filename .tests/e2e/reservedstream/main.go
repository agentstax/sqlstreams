package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	iCommon "github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/metric"
	metricscontroller "github.com/agentstax/sqlstreams/pkg/metric/controller"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/agentstax/sqlstreams/pkg/stream"
)

func main() {
	if err := run(); err != nil {
		fmt.Printf("\n❌ E2E TEST FAILED: %s\n", err.Error())
		os.Exit(1)
	}
}

func run() (err error) {
	defer common.Recover(&err)
	ctx := context.Background()
	run := time.Now().UnixNano()

	pool, err := common.NewPool(ctx, nil)
	common.Must(err)
	defer pool.Close()

	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	common.Must(err)

	step("RegisterSystem creates __system.metrics idempotently")
	common.Must(client.System().Register(ctx, nil))
	metricStream, err := client.Stream[sqlstreams.RawPayload](metric.MetricStreamName).Get(ctx)
	common.Must(err)
	if metricStream == nil {
		common.Die("expected __system.metrics to exist after RegisterSystem")
	}
	fmt.Printf("  ✓ __system.metrics exists, id=%d, retention=%v, partition_size=%d, delivery_log=%s\n",
		metricStream.Id, metricStream.RetentionTTL, metricStream.PartitionSize, metricStream.DeliveryLogMode)

	step("RegisterStream rejects a user name under the reserved prefix")
	_, err = client.Stream[sqlstreams.RawPayload](iCommon.SystemStreamPrefix+"evil").Register(ctx, nil)
	assertReserved("RegisterStream(__system.evil)", err)

	step("RenameStream refused both directions")
	_, err = client.Stream[sqlstreams.RawPayload](metric.MetricStreamName).Rename(ctx, fmt.Sprintf("reservedstream.stolen.%d", run))
	assertReserved("RenameStream(__system.metrics -> user name)", err)

	userStream, err := client.Stream[sqlstreams.RawPayload](fmt.Sprintf("reservedstream.user.%d", run)).Register(ctx, nil)
	common.Must(err)
	_, err = client.Stream[sqlstreams.RawPayload](userStream.Name).Rename(ctx, iCommon.SystemStreamPrefix+"evil")
	assertReserved("RenameStream(user name -> __system.evil)", err)
	common.Must(client.Stream[sqlstreams.RawPayload](userStream.Name).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true}))

	step("DestroyStream refused on the system stream")
	err = client.Stream[sqlstreams.RawPayload](metric.MetricStreamName).Destroy(ctx, &sqlstreams.DestroyOptions{Force: true})
	assertReserved("DestroyStream(__system.metrics)", err)

	step("re-running RegisterSystem keeps the same row and re-declares its config")
	common.Must(client.System().Register(ctx, nil))
	afterRerun, err := client.Stream[sqlstreams.RawPayload](metric.MetricStreamName).Get(ctx)
	common.Must(err)
	assertInt64("stream id unchanged across re-run", afterRerun.Id, metricStream.Id)
	assertDuration("declared retention across re-run", afterRerun.RetentionTTL, metricscontroller.StreamConfig().RetentionTTL)

	fmt.Println("\n✅ RESERVED STREAM E2E TEST PASSED")
	return nil
}

func assertReserved(label string, err error) {
	if !errors.Is(err, stream.ErrReservedStreamName) {
		common.Die(fmt.Sprintf("%s: expected ErrReservedStreamName, got %v", label, err))
	}
	fmt.Printf("  ✓ %s -> %v\n", label, err)
}

func assertDuration(label string, got, want time.Duration) {
	if got != want {
		common.Die(fmt.Sprintf("%s: got %v, want %v", label, got, want))
	}
	fmt.Printf("  ✓ %s (%v)\n", label, got)
}

func assertInt64(label string, got, want int64) {
	if got != want {
		common.Die(fmt.Sprintf("%s: got %d, want %d", label, got, want))
	}
	fmt.Printf("  ✓ %s (%d)\n", label, got)
}

func step(s string) { fmt.Printf("\n--- %s ---\n", s) }
