package manager

import (
	"fmt"
	"math"
	"time"
)

type ManagerConfig struct {
	Execution         string
	TimeScale         float64
	DrainBudget       time.Duration
	Repetitions       int
	Replicas          int
	SynchronousCommit string
	ResultsDir        string
}

func (m *ManagerConfig) WithDefaults() *ManagerConfig {
	if m.Execution == "" {
		m.Execution = "compose"
	}
	if m.TimeScale == 0 {
		m.TimeScale = 1
	}
	if m.DrainBudget == 0 {
		m.DrainBudget = 2 * time.Minute
	}
	if m.Repetitions == 0 {
		m.Repetitions = 1
	}
	if m.Replicas == 0 {
		m.Replicas = 1
	}
	if m.SynchronousCommit == "" {
		m.SynchronousCommit = "on"
	}
	if m.ResultsDir == "" {
		m.ResultsDir = "results"
	}
	return m
}

func (m *ManagerConfig) Validate() error {
	if m.Execution != "native" && m.Execution != "compose" {
		return fmt.Errorf("Execution must be native or compose, got %q", m.Execution)
	}
	if m.TimeScale <= 0 || math.IsNaN(m.TimeScale) || math.IsInf(m.TimeScale, 0) {
		return fmt.Errorf("TimeScale must be finite and > 0, got %g", m.TimeScale)
	}
	if m.DrainBudget <= 0 {
		return fmt.Errorf("DrainBudget must be > 0, got %v", m.DrainBudget)
	}
	if m.Repetitions < 1 {
		return fmt.Errorf("Repetitions must be >= 1, got %d", m.Repetitions)
	}
	if m.Replicas < 1 {
		return fmt.Errorf("Replicas must be >= 1, got %d", m.Replicas)
	}
	if m.SynchronousCommit != "on" && m.SynchronousCommit != "off" {
		return fmt.Errorf("SynchronousCommit must be on or off, got %q", m.SynchronousCommit)
	}
	if m.Execution == "native" {
		if m.SynchronousCommit != "on" {
			return fmt.Errorf("native execution requires SynchronousCommit on, got %q", m.SynchronousCommit)
		}
		if m.Repetitions != 1 {
			return fmt.Errorf("native execution uses one supplied empty database; Repetitions must be 1")
		}
	}
	return nil
}
