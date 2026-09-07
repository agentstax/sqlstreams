package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/agentstax/vulkan/bench/reliability/scenarios"
)

type labFlags struct {
	role      string
	scenario  string
	timeScale float64
	ledgerDir string
	name      string
}

func parseFlags() (*labFlags, error) {
	flags := &labFlags{}
	flag.StringVar(&flags.role, "role", "print", "producer, consumer, or print")
	flag.StringVar(&flags.scenario, "scenario", "dev", "scenario to run: "+scenarios.Names())
	flag.Float64Var(&flags.timeScale, "time-scale", 1, "multiplier on every phase duration and offset; 1/60 runs the hour in a minute")
	flag.StringVar(&flags.ledgerDir, "ledger-dir", "ledger", "directory the role's ledger files are appended under")
	flag.StringVar(&flags.name, "name", "", "this process's name in the ledger; default the hostname")
	flag.Parse()

	if flags.timeScale <= 0 {
		return nil, fmt.Errorf("time-scale must be > 0, got %g", flags.timeScale)
	}
	if flags.name == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, err
		}
		flags.name = hostname
	}
	return flags, nil
}
