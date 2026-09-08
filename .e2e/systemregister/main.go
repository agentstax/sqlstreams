package main

// systemregister is the go-forward dev bootstrap: it stands up the shared
// control-plane tables in Go (RegisterSystem), superseding the golang-migrate
// migrate-up path. Idempotent -- safe to re-run.

import (
	"context"
	"fmt"
	"os"

	vulkan "github.com/agentstax/vulkan/pkg/vulkan"
)

func main() {
	if err := run(); err != nil {
		fmt.Printf("\n❌ E2E TEST FAILED: %s\n", err.Error())
		os.Exit(1)
	}
}

// testFailure is what die panics with; run recovers it into its error so
// main's deferred cleanup runs on a failed assertion.
type testFailure struct {
	message string
}

func (f testFailure) Error() string {
	return f.message
}

func run() (err error) {
	defer func() {
		switch recovered := recover().(type) {
		case nil:
		case testFailure:
			err = recovered
		default:
			panic(recovered)
		}
	}()
	ctx := context.Background()

	pool, err := vulkan.NewPostgresPool(ctx, "example_user", "example_password", "localhost", "example_db", nil)
	must(err)
	defer pool.Close()

	client, err := vulkan.NewClient(ctx, pool, nil)
	must(err)

	must(client.System().Register(ctx, nil))
	fmt.Println("system registered")
	return nil
}

func must(err error) {
	if err != nil {
		die(err.Error())
	}
}

func die(msg string) {
	panic(testFailure{message: msg})
}
