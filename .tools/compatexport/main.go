package main

// compat export: writes the doc site's compatibility matrix as JSON. Every
// cell is decided by migrate.ClassifySchemaSupport -- the same call the
// library makes at Register -- so the page can render the gate without
// holding a second copy of its rule.

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	streamMigrations "github.com/agentstax/sqlstreams/pkg/stream/migrations"
	systemMigrations "github.com/agentstax/sqlstreams/pkg/system/migrations"
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

	export, err := NewExport(systemMigrations.Registry, streamMigrations.Registry)
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
