package main

import (
	"context"
	"os"

	"github.com/allegedlyreliable/sqlstreams/cmd/sqlstreams/internal/cli"
)

// GoReleaser sets version; an empty value lets Fang read the Go module version.
var version string

func main() {
	os.Exit(cli.Execute(context.Background(), version))
}
