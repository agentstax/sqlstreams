package main

// holdingproducer produces one message inside a transaction it never ends,
// prints "holding", and waits to be killed.

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	"github.com/agentstax/sqlstreams/client"
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
	ctx := context.Background()
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

	return client.InTransaction(ctx, func(ctx context.Context, tx sqlstreams.Tx) error {
		work, err := common.NewWork(30, "admin@example.com")
		if err != nil {
			return err
		}
		if _, err := producer.ProduceInTx(ctx, tx, work, nil); err != nil {
			return err
		}
		fmt.Println("holding")
		select {}
	})
}
