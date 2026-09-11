package main

// loopingproducer produces until the lifecycle context ends, printing
// "produced <id>" once each produce call has returned. Each produce runs
// under its own context: cancelling a produce's ctx stops the wait, not the
// message, so a message is reported only once its call returned.

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
)

var streamName = flag.String("stream", "", "the stream to produce on")

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
	producer, err := client.Stream[common.Work](*streamName).Producer().Register(ctx, nil)
	if err != nil {
		return err
	}

	for ctx.Err() == nil {
		work, err := common.NewWork(30, "admin@example.com")
		if err != nil {
			return err
		}
		produced, err := producer.Produce(context.Background(), work, nil)
		if err != nil {
			return err
		}
		fmt.Printf("produced %d\n", produced.Id)
	}
	return nil
}
