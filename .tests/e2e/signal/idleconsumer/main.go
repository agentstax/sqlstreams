package main

// idleconsumer prints "consuming" and consumes under the lifecycle context
// until it ends; its handler returns at once.

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	"github.com/agentstax/sqlstreams/client"
)

var streamName = flag.String("stream", "", "the stream to consume")

func main() {
	flag.Parse()
	if err := run(); err != nil {
		fmt.Printf("\n❌ E2E TEST FAILED: %s\n", err.Error())
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := sqlstreams.LifecycleContext(nil)
	defer stop()
	pool, err := common.NewPool(ctx, nil)
	if err != nil {
		return err
	}
	defer pool.Close()
	client, err := sqlstreams.NewClient(ctx, pool, nil)
	if err != nil {
		return err
	}
	consumer, err := client.Stream[common.Work](*streamName).Consumer("signal.e2e").Register(ctx, nil)
	if err != nil {
		return err
	}

	fmt.Println("consuming")
	err = consumer.Consume(ctx, handle, &sqlstreams.ConsumeOptions{ClaimPollRate: 200 * time.Millisecond})
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func handle(ctx context.Context, work *common.Work) error {
	return nil
}
