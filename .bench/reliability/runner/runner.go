package runner

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/agentstax/sqlstreams/.bench/reliability/common"
	"github.com/agentstax/sqlstreams/.bench/reliability/record"
	"github.com/agentstax/sqlstreams/.bench/reliability/runner/datastore"
	"github.com/agentstax/sqlstreams/.bench/reliability/scenario"
)

// Runner runs one role of one scenario: it registers what the scenario
// declares, walks the scenario's timeline, writes the run_phase rows, and
// drives the recording producer or consumer instances at the moments the
// timeline says. The producer and consumer packages never see the timeline.
type Runner struct {
	ds         *datastore.RunnerDatastore
	declared   *scenario.Scenario
	unscaled   *scenario.Scenario
	timeScale  float64
	connection *common.Connection
	recordDir  string
	name       string
}

// NewRunner runs the declared scenario scaled by timeScale; the roles run
// the scaled timeline, and the checker records the declaration as written.
func NewRunner(declared *scenario.Scenario, timeScale float64, connection *common.Connection, recordDir string, name string) (*Runner, error) {
	if declared == nil {
		return nil, errors.New("declared must not be nil")
	}
	if timeScale <= 0 {
		return nil, fmt.Errorf("timeScale must be > 0, got %g", timeScale)
	}
	if connection == nil {
		return nil, errors.New("connection must not be nil")
	}
	if recordDir == "" {
		return nil, errors.New("recordDir must not be empty")
	}
	if name == "" {
		return nil, errors.New("name must not be empty")
	}
	scaled := declared.Scaled(timeScale)
	if err := scaled.Validate(); err != nil {
		return nil, err
	}
	ds, err := datastore.NewRunnerDatastore(connection.Pool)
	if err != nil {
		return nil, err
	}
	return &Runner{ds: ds, declared: scaled, unscaled: declared, timeScale: timeScale, connection: connection, recordDir: recordDir, name: name}, nil
}

func (r *Runner) openWriter(kind record.FileKind) (*record.Writer, error) {
	return record.NewWriter(r.recordDir, r.name, kind)
}

func (r *Runner) writePhase(phaseRecords *record.Writer, kind record.PhaseKind, name string, status record.PhaseStatus, detail string) error {
	return phaseRecords.Write(record.PhaseRecord{
		At:      time.Now(),
		Process: r.name,
		Kind:    kind,
		Name:    name,
		Status:  status,
		Detail:  detail,
	})
}

// ***************
// *** HELPERS ***
// ***************

// ignoreCancellation turns the lifecycle ctx's own cancellation -- SIGTERM
// from compose -- into a clean return; any other error is a lab failure.
func ignoreCancellation(err error) error {
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
