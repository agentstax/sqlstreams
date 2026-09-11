package main

// systemregister is the go-forward dev bootstrap: it stands up the shared
// control-plane tables in Go (RegisterSystem), superseding the golang-migrate
// migrate-up path. Idempotent -- safe to re-run.

import (
	"context"
	"fmt"
	"os"

	"github.com/agentstax/sqlstreams/.tests/e2e/common"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
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

	pool, err := common.NewPool(ctx, nil)
	common.Must(err)
	defer pool.Close()

	client, err := sqlstreams.NewClient(ctx, pool, nil)
	common.Must(err)

	common.Must(client.System().Register(ctx, nil))
	fmt.Println("system registered")
	return nil
}
