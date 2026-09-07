package runner

import (
	"context"
	"errors"
	"time"

	"github.com/agentstax/vulkan/bench/reliability/common"
	"github.com/agentstax/vulkan/bench/reliability/ledger"
	"github.com/agentstax/vulkan/bench/reliability/scenario"
)

// Runner runs one role of one scenario: it registers what the scenario
// declares, walks the scenario's timeline, writes the run_phase facts, and
// drives the verifiable producer or consumer fleet at the moments the
// timeline says. The producer and consumer packages never see the timeline.
type Runner struct {
	declared   *scenario.Scenario
	connection *common.Connection
	ledgerDir  string
	name       string
}

func NewRunner(declared *scenario.Scenario, connection *common.Connection, ledgerDir string, name string) (*Runner, error) {
	if declared == nil {
		return nil, errors.New("declared must not be nil")
	}
	if connection == nil {
		return nil, errors.New("connection must not be nil")
	}
	if ledgerDir == "" {
		return nil, errors.New("ledgerDir must not be empty")
	}
	if name == "" {
		return nil, errors.New("name must not be empty")
	}
	return &Runner{declared: declared, connection: connection, ledgerDir: ledgerDir, name: name}, nil
}

func (r *Runner) openLedger(kind ledger.FileKind) (*ledger.Writer, error) {
	return ledger.NewWriter(r.ledgerDir, r.name, kind)
}

func (r *Runner) writePhase(phases *ledger.Writer, kind ledger.PhaseKind, name string, status ledger.PhaseStatus, detail string) error {
	return phases.Write(ledger.PhaseFact{
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
