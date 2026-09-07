package main

// reliability lab: an hour of real producers and consumers against a real
// Postgres, ending in one verdict. Every produce and every handler invocation
// is written to a record file; after producers stop and consumers drain, the
// checker joins those records against Vulkan's own tables and sorts every
// message into a named bucket. Design in decision record 0687; the proposal
// page is website/src/content/docs/concepts/reliability-lab.mdx.
//
// One binary, one role per process: -role producer walks the scenario's
// phases, -role consumer follows its consumer timeline until stopped, -role
// checker judges the run and exits with the verdict (0 pass, 1 fail, 2
// unknown), and -role print writes the scenario in its .scenario format.
// Exit 3 is a lab failure (connection, flags, records), never a verdict.

import (
	"fmt"
	"os"

	"github.com/agentstax/vulkan/bench/reliability/common"
	"github.com/agentstax/vulkan/bench/reliability/runner"
	"github.com/agentstax/vulkan/bench/reliability/scenarios"
	vulkan "github.com/agentstax/vulkan/pkg/vulkan"
)

// exitLabFailure is the one exit code that is not a verdict: connection,
// flags, or record files failed before or columns any judging.
const exitLabFailure = 3

func main() {
	code, err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitLabFailure)
	}
	os.Exit(code)
}

// run returns the process exit code: the verdict's for the checker, 0 for
// every other role that finished.
func run() (int, error) {
	flags, err := parseFlags()
	if err != nil {
		return 0, err
	}
	declared, ok := scenarios.ByName(flags.scenario)
	if !ok {
		return 0, fmt.Errorf("unrecognized scenario: %q -- one of %s", flags.scenario, scenarios.Names())
	}
	if flags.role == "print" {
		fmt.Print(declared.String())
		return 0, nil
	}

	ctx, stop := vulkan.LifecycleContext(nil)
	defer stop()
	connection, err := common.NewConnection(ctx)
	if err != nil {
		return 0, err
	}
	defer connection.Close()
	role, err := runner.NewRunner(declared.Scaled(flags.timeScale), connection, flags.recordDir, flags.name)
	if err != nil {
		return 0, err
	}

	switch flags.role {
	case "producer":
		return 0, role.RunProducer(ctx)
	case "consumer":
		return 0, role.RunConsumer(ctx)
	case "checker":
		verdict, err := role.RunChecker(ctx, flags.resultsDir, flags.drainBudget)
		if err != nil {
			return 0, err
		}
		return verdict.ExitCode(), nil
	}
	return 0, fmt.Errorf("unrecognized role: %q -- one of producer, consumer, checker, print", flags.role)
}
