package main

// reliability lab: an hour of real producers and consumers against a real
// Postgres, ending in one verdict. Every produce and every handler invocation
// is written to a ledger; after producers stop and consumers drain, the
// checker joins the ledger against Vulkan's own tables and sorts every
// message into a named bucket. Design in decision record 0687; the proposal
// page is website/src/content/docs/concepts/reliability-lab.mdx.
//
// One binary, one role per process: -role producer walks the scenario's
// phases, -role consumer follows its consumer timeline until stopped, and
// -role print writes the scenario in its .scenario format. Exit 3 is a lab
// failure (connection, flags, ledger), never a verdict.

import (
	"fmt"
	"os"

	"github.com/agentstax/vulkan/bench/reliability/coordinator"
	"github.com/agentstax/vulkan/bench/reliability/lab"
	"github.com/agentstax/vulkan/bench/reliability/scenarios"
	vulkan "github.com/agentstax/vulkan/pkg/vulkan"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
}

func run() error {
	flags, err := parseFlags()
	if err != nil {
		return err
	}
	declared, ok := scenarios.ByName(flags.scenario)
	if !ok {
		return fmt.Errorf("unrecognized scenario: %q -- one of %s", flags.scenario, scenarios.Names())
	}
	if flags.role == "print" {
		fmt.Print(declared.String())
		return nil
	}

	ctx, stop := vulkan.LifecycleContext(nil)
	defer stop()
	connection, err := lab.NewConnection(ctx)
	if err != nil {
		return err
	}
	defer connection.Close()
	run, err := coordinator.NewCoordinator(declared.Scaled(flags.timeScale), connection, flags.ledgerDir, flags.name)
	if err != nil {
		return err
	}

	switch flags.role {
	case "producer":
		return run.RunProducer(ctx)
	case "consumer":
		return run.RunConsumer(ctx)
	}
	return fmt.Errorf("unrecognized role: %q -- one of producer, consumer, print", flags.role)
}
