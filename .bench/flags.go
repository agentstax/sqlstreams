package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/allegedlyreliable/sqlstreams/.bench/manager"
	"github.com/allegedlyreliable/sqlstreams/.bench/scenarios"
)

type labFlags struct {
	manager      *manager.ManagerConfig
	role         string
	scenario     string
	scenarioFile string
	timeScale    float64
	recordDir    string
	name         string

	runDir          string
	resultsDir      string
	fingerprintFile string
	statsFile       string
	drainBudget     time.Duration
}

func parseFlags() (*labFlags, error) {
	flags := &labFlags{manager: &manager.ManagerConfig{}}
	flag.StringVar(&flags.role, "role", "print", "manager, producer, consumer, observer, checker, report, print, or print-json")
	flag.StringVar(&flags.scenario, "scenario", "quiet", "scenario to run: "+scenarios.Names())
	flag.StringVar(&flags.scenarioFile, "scenario-file", os.Getenv("SCENARIO_FILE"), "JSON scenario declaration; overrides -scenario")
	flag.Float64Var(&flags.timeScale, "time-scale", 1, "multiplier on every phase duration and offset; 1/60 runs the hour in a minute")
	flag.StringVar(&flags.recordDir, "record-dir", "records", "directory the role's record files are appended under")
	flag.StringVar(&flags.name, "name", "", "this process's name in the records; default the hostname")
	flag.StringVar(&flags.runDir, "run-dir", "", "checker: existing run directory provided by manager")
	flag.StringVar(&flags.resultsDir, "results-dir", "results", "manager, checker, and report: results directory")
	flag.StringVar(&flags.fingerprintFile, "fingerprint-file", "results/fingerprint.json", "checker: the environment record fingerprint.sh wrote before the run")
	flag.StringVar(&flags.statsFile, "stats-file", "results/stats.jsonl", "checker: CPU samples recorded during the run")
	flag.DurationVar(&flags.drainBudget, "drain-budget", 2*time.Minute, "checker: how long to wait for the consumers to finish before the verdict is unknown")
	flag.StringVar(&flags.manager.Execution, "execution", "compose", "manager: native or compose")
	flag.IntVar(&flags.manager.Repetitions, "reps", 1, "manager: repetitions with fresh databases")
	flag.IntVar(&flags.manager.Replicas, "replicas", 1, "manager: consumer processes")
	flag.StringVar(&flags.manager.SynchronousCommit, "sync", "on", "manager: synchronous_commit (off requires Compose)")
	flag.Parse()
	flags.manager.TimeScale = flags.timeScale
	flags.manager.DrainBudget = flags.drainBudget
	flags.manager.ResultsDir = flags.resultsDir

	if flags.role == "checker" && flags.runDir == "" {
		return nil, fmt.Errorf("checker requires -run-dir")
	}
	if flags.timeScale <= 0 || math.IsNaN(flags.timeScale) || math.IsInf(flags.timeScale, 0) {
		return nil, fmt.Errorf("time-scale must be finite and > 0, got %g", flags.timeScale)
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
