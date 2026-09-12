package main

// The benchmark runner drives scenario-declared producers and consumers against
// Postgres and ends each run with a checker verdict. Scenarios choose full
// per-message evidence or aggregate counters.
//
// Manager coordinates complete runs; each child executes one role.
// One binary, one role per process: -role producer walks the scenario's
// phases, -role consumer follows its consumer timeline until stopped, -role
// observer samples the server once a second until stopped, -role checker
// judges the run and exits with the verdict (0 pass, 1 fail, 2 unknown),
// -role report summarizes the scenario's recorded runs, and -role print
// writes the scenario in its .scenario format.
// Exit 3 is a lab failure (connection, flags, records), never a verdict.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/agentstax/sqlstreams/.bench/checker"
	"github.com/agentstax/sqlstreams/.bench/common"
	"github.com/agentstax/sqlstreams/.bench/manager"
	"github.com/agentstax/sqlstreams/.bench/runner"
	"github.com/agentstax/sqlstreams/.bench/scenario"
	"github.com/agentstax/sqlstreams/.bench/scenarios"
	"github.com/agentstax/sqlstreams/client"
)

// exitLabFailure is the one exit code that is not a verdict: connection,
// flags, or record files failed before judging.
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
	if flags.role == "report" {
		runs, err := checker.ReadRuns(flags.resultsDir, flags.scenario)
		if err != nil {
			return 0, err
		}
		fmt.Print(checker.RunsReport(flags.scenario, runs))
		return 0, nil
	}

	declared, ok := scenarios.ByName(flags.scenario)
	if flags.scenarioFile != "" {
		encoded, err := os.ReadFile(flags.scenarioFile)
		if err != nil {
			return 0, err
		}
		declared = &scenario.Scenario{}
		decoder := json.NewDecoder(bytes.NewReader(encoded))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(declared); err != nil {
			return 0, err
		}
		if err := declared.Validate(); err != nil {
			return 0, err
		}
		ok = true
	}
	if !ok {
		return 0, fmt.Errorf("unrecognized scenario: %q -- one of %s", flags.scenario, scenarios.Names())
	}
	if flags.role == "print-json" {
		return 0, json.NewEncoder(os.Stdout).Encode(declared)
	}
	if flags.role == "print" {
		fmt.Print(declared.String())
		return 0, nil
	}

	ctx, stop := sqlstreams.LifecycleContext(nil)
	defer stop()
	if flags.role == "manager" {
		instance, err := manager.NewManager(declared, flags.manager)
		if err != nil {
			return 0, err
		}
		return instance.Run(ctx)
	}
	connection, err := common.NewConnection(ctx, declared.MaxConns)
	if err != nil {
		return 0, err
	}
	defer connection.Close()
	role, err := runner.NewRunner(declared, flags.timeScale, connection, flags.recordDir, flags.name)
	if err != nil {
		return 0, err
	}

	switch flags.role {
	case "producer":
		return 0, role.RunProducer(ctx)
	case "consumer":
		return 0, role.RunConsumer(ctx)
	case "observer":
		return 0, role.RunObserver(ctx)
	case "checker":
		verdict, err := role.RunChecker(ctx, flags.runDir, flags.resultsDir, flags.fingerprintFile, flags.statsFile, flags.drainBudget)
		if err != nil {
			return 0, err
		}
		return verdict.ExitCode(), nil
	}
	return 0, fmt.Errorf("unrecognized role: %q -- one of manager, producer, consumer, observer, checker, report, print", flags.role)
}
