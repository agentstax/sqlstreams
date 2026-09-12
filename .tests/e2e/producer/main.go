package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"

	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	sqlstreams "github.com/agentstax/sqlstreams/client"
)

func main() {
	if err := run(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func run() error {
	// FLAGS

	// -count n
	countPtr := flag.Int("count", 1, "number of messages produced")

	// -routing-key key
	routingKeyPtr := flag.String("routing-key", "", "routing key attached to each message (optional)")

	// -stream name
	streamPtr := flag.String("stream", "learning.v1", "stream to publish to (this command declares it -- the other examples only read it)")

	// must always parse
	flag.Parse()

	fmt.Printf("count: %d, routing-key: %q, stream: %q\n", *countPtr, *routingKeyPtr, *streamPtr)

	// SETUP
	ctx := context.Background()

	pool, err := common.NewPool(ctx, nil)
	if err != nil {
		return err
	}
	defer pool.Close()

	client, err := sqlstreams.NewClient(ctx, pool, &sqlstreams.ClientConfig{AllowDestroy: true})
	if err != nil {
		return err
	}

	t, err := client.Stream[sqlstreams.RawPayload](*streamPtr).Register(ctx, &sqlstreams.StreamConfig{})
	if err != nil {
		return err
	}

	wpInstance, err := client.Stream[common.Work](t.Name).Producer().Register(ctx, nil)
	if err != nil {
		return err
	}

	// WORK
	for range *countPtr {
		produced, err := wpInstance.ProduceFunc(ctx, func(ctx context.Context, tx sqlstreams.Tx) (*common.Work, error) {
			work, err := common.NewWork(rand.IntN(100), "admin@example.com")
			if err != nil {
				return nil, err
			}

			return work, nil
		}, &sqlstreams.ProduceOptions{RoutingKey: *routingKeyPtr})
		if err != nil {
			return err
		}

		fmt.Printf("successfully produced message: work=%s id=%d\n", produced.Message.Id, produced.Id)
	}
	return nil
}
