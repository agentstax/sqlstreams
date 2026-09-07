package checker

import (
	"context"
	"fmt"
	"time"

	"github.com/agentstax/vulkan/bench/reliability/checker/datastore"
	"github.com/agentstax/vulkan/bench/reliability/record"
	"github.com/agentstax/vulkan/bench/reliability/scenario"
)

// scheduleTolerance is how far behind its scheduled instant a produce may
// start before its second counts against schedule_kept.
const scheduleTolerance = 100 * time.Millisecond

// MeasureSummary is what every run measures, whatever it declares: latency
// from scheduled time over the whole run and per phase, and the committed
// produces per second. All of it is computed by the server over the record
// rows; nothing is merged from per-process numbers.
type MeasureSummary struct {
	Produce    datastore.LatencySummary     `json:"produce"`
	EndToEnd   datastore.LatencySummary     `json:"end_to_end"`
	Phases     []PhaseSummary               `json:"phases"`
	Throughput []datastore.ThroughputSample `json:"throughput"`
}

// PhaseSummary is one producer phase measured: the rate it achieved against
// the rate it declared, and the latency of the produces it scheduled.
type PhaseSummary struct {
	Name         string                   `json:"name"`
	DeclaredRate int                      `json:"declared_rate"`
	AchievedRate float64                  `json:"achieved_rate"`
	Produce      datastore.LatencySummary `json:"produce"`
	EndToEnd     datastore.LatencySummary `json:"end_to_end"`
}

// measure reads the run's latency and throughput, per producer phase and
// whole, from the phase rows' windows.
func (c *Checker) measure(ctx context.Context, phases []record.PhaseRecord) (*MeasureSummary, error) {
	summary := &MeasureSummary{Phases: []PhaseSummary{}}
	var runStart, runEnd time.Time
	for _, phase := range c.declared.Producer {
		from, to, err := phaseWindow(phases, phase)
		if err != nil {
			return nil, err
		}
		if runStart.IsZero() || from.Before(runStart) {
			runStart = from
		}
		if to.After(runEnd) {
			runEnd = to
		}

		measured, err := c.measurePhase(ctx, phase, from, to)
		if err != nil {
			return nil, err
		}
		summary.Phases = append(summary.Phases, measured)
	}

	var err error
	summary.Produce, err = c.ds.ReadProduceLatency(ctx, runStart, runEnd)
	if err != nil {
		return nil, err
	}
	summary.EndToEnd, err = c.ds.ReadEndToEndLatency(ctx, runStart, runEnd)
	if err != nil {
		return nil, err
	}
	summary.Throughput, err = c.ds.ReadThroughput(ctx, runStart, runEnd)
	if err != nil {
		return nil, err
	}
	return summary, nil
}

func (c *Checker) measurePhase(ctx context.Context, phase scenario.ProducerPhase, from time.Time, to time.Time) (PhaseSummary, error) {
	measured := PhaseSummary{Name: phase.Name, DeclaredRate: phase.Rate}
	var err error
	measured.Produce, err = c.ds.ReadProduceLatency(ctx, from, to)
	if err != nil {
		return PhaseSummary{}, err
	}
	measured.EndToEnd, err = c.ds.ReadEndToEndLatency(ctx, from, to)
	if err != nil {
		return PhaseSummary{}, err
	}
	measured.AchievedRate = float64(measured.Produce.Count) / to.Sub(from).Seconds()
	return measured, nil
}

// ***************
// *** HELPERS ***
// ***************

// phaseWindow is [started, ended) of one producer phase from its run_phase
// rows; a phase missing either row was cut short, and the run cannot be
// measured.
func phaseWindow(phases []record.PhaseRecord, phase scenario.ProducerPhase) (time.Time, time.Time, error) {
	var started, ended time.Time
	for _, row := range phases {
		if row.Kind != record.PhaseKindProducer || row.Name != phase.Name {
			continue
		}
		switch row.Status {
		case record.PhaseStatusStarted:
			started = row.At
		case record.PhaseStatusEnded:
			ended = row.At
		}
	}
	if started.IsZero() || ended.IsZero() {
		return time.Time{}, time.Time{}, fmt.Errorf("phase %q has no started and ended rows -- the producer did not finish it", phase.Name)
	}
	return started, ended, nil
}
