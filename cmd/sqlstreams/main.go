package main

import (
	"context"
	"os"

	"github.com/allegedlyreliable/sqlstreams/cmd/sqlstreams/internal/cli"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(cli.Execute(context.Background(), version))
}
