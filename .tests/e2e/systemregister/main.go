package main

// systemregister is the go-forward dev bootstrap: it stands up the shared
// control-plane tables in Go (RegisterSystem), superseding the golang-migrate
// migrate-up path. Idempotent -- safe to re-run.

import (
	"context"
	"fmt"
	"os"

	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	"github.com/agentstax/sqlstreams/client"
)

func main() {
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

	if err := client.System().Register(ctx, nil); err != nil {
		return err
	}
	fmt.Println("system registered")
	return nil
}
