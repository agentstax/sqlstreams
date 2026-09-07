package main

// reliability lab: an hour of real producers and consumers against a real
// Postgres, ending in one verdict. Every produce and every handler invocation
// is written to a ledger; after producers stop and consumers drain, the
// checker joins the ledger against Vulkan's own tables and sorts every
// message into a named bucket. Design in decision record 0687; the proposal
// page is website/src/content/docs/concepts/reliability-lab.mdx.
//
// Step 1 ships the scenario declarations and their printer. `-scenario`
// prints the named scenario in the .scenario format the report will echo.

import (
	"flag"
	"fmt"
	"os"

	"github.com/agentstax/vulkan/bench/reliability/scenarios"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
}

func run() error {
	name := flag.String("scenario", "dev", "scenario to print: "+scenarios.Names())
	flag.Parse()

	scenario, ok := scenarios.ByName(*name)
	if !ok {
		return fmt.Errorf("unrecognized scenario: %q -- one of %s", *name, scenarios.Names())
	}
	fmt.Print(scenario.String())
	return nil
}
