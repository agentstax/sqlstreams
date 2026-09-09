package main

// code export: writes every SS-coded declaration as JSON for the doc site.
// The declarations are the source -- the site renders hand-written prose and
// reads this only for what a page cannot restate by hand: the diagnose
// queries, and the record each page's frontmatter is checked against.

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/agentstax/sqlstreams/pkg/common/diagnostic"

	// declaring packages, linked so the registry this binary sees is
	// complete. .tools/conventions holds the walk that proves it.
	_ "github.com/agentstax/sqlstreams/pkg/alert"
	_ "github.com/agentstax/sqlstreams/pkg/common"
	_ "github.com/agentstax/sqlstreams/pkg/compaction"
	_ "github.com/agentstax/sqlstreams/pkg/consume"
	_ "github.com/agentstax/sqlstreams/pkg/metric"
	_ "github.com/agentstax/sqlstreams/pkg/migrate"
	_ "github.com/agentstax/sqlstreams/pkg/produce"
	_ "github.com/agentstax/sqlstreams/pkg/schedule"
	_ "github.com/agentstax/sqlstreams/pkg/stream"
	_ "github.com/agentstax/sqlstreams/pkg/system"
	_ "github.com/agentstax/sqlstreams/pkg/worker"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	out := flag.String("out", "", "file to write the JSON to; empty writes to stdout")
	flag.Parse()

	export, err := NewExport(diagnostic.Errors(), diagnostic.Events(), diagnostic.Metrics(), diagnostic.Alerts())
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')

	if *out == "" {
		_, err = os.Stdout.Write(encoded)
		return err
	}
	return os.WriteFile(*out, encoded, 0o644)
}
