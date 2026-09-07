package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/agentstax/vulkan/bench/reliability/scenarios"
)

type labFlags struct {
	role      string
	scenario  string
	timeScale float64
	recordDir string
	name      string

	resultsDir      string
	fingerprintFile string
	drainBudget     time.Duration
}

func parseFlags() (*labFlags, error) {
	flags := &labFlags{}
	flag.StringVar(&flags.role, "role", "print", "producer, consumer, checker, or print")
	flag.StringVar(&flags.scenario, "scenario", "dev", "scenario to run: "+scenarios.Names())
	flag.Float64Var(&flags.timeScale, "time-scale", 1, "multiplier on every phase duration and offset; 1/60 runs the hour in a minute")
	flag.StringVar(&flags.recordDir, "record-dir", "records", "directory the role's record files are appended under")
	flag.StringVar(&flags.name, "name", "", "this process's name in the records; default the hostname")
	flag.StringVar(&flags.resultsDir, "results-dir", "results", "checker: directory the verdict record is written under")
	flag.StringVar(&flags.fingerprintFile, "fingerprint-file", "results/fingerprint.json", "checker: the environment record fingerprint.sh wrote before the run")
	flag.DurationVar(&flags.drainBudget, "drain-budget", 2*time.Minute, "checker: how long to wait for the consumers to finish before the verdict is unknown")
	flag.Parse()

	if flags.timeScale <= 0 {
		return nil, fmt.Errorf("time-scale must be > 0, got %g", flags.timeScale)
	}
	if flags.drainBudget <= 0 {
		return nil, fmt.Errorf("drain-budget must be > 0, got %v", flags.drainBudget)
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
