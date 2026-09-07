package runner

import (
	"context"
	"errors"
	"time"

	"github.com/agentstax/vulkan/bench/reliability/common"
	"github.com/agentstax/vulkan/bench/reliability/record"
	"github.com/agentstax/vulkan/bench/reliability/scenario"
)

// Runner runs one role of one scenario: it registers what the scenario
// declares, walks the scenario's timeline, writes the run_phase facts, and
// drives the verifiable producer or consumer fleet at the moments the
// timeline says. The producer and consumer packages never see the timeline.
type Runner struct {
	declared   *scenario.Scenario
	connection *common.Connection
	recordDir  string
	name       string
}

func NewRunner(declared *scenario.Scenario, connection *common.Connection, recordDir string, name string) (*Runner, error) {
	if declared == nil {
		return nil, errors.New("declared must not be nil")
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
	return &Runner{declared: declared, connection: connection, recordDir: recordDir, name: name}, nil
}

func (r *Runner) openWriter(kind record.FileKind) (*record.Writer, error) {
	return record.NewWriter(r.recordDir, r.name, kind)
}

func (r *Runner) writePhase(phases *record.Writer, kind record.PhaseKind, name string, status record.PhaseStatus, detail string) error {
	return phases.Write(record.Phase{
		At:     time.Now(),
		Role:   r.name,
		Kind:   kind,
		Name:   name,
		Status: status,
		Detail: detail,
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
