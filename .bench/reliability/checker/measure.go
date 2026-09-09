package checker

import (
	"context"
	"fmt"
	"time"

	"github.com/agentstax/sqlstreams/.bench/reliability/checker/datastore"
	"github.com/agentstax/sqlstreams/.bench/reliability/record"
	"github.com/agentstax/sqlstreams/.bench/reliability/scenario"
)

// scheduleTolerance is how far behind its scheduled instant a produce may
// start before its second counts against schedule_kept.
const scheduleTolerance = 100 * time.Millisecond

// backlogSlopeFraction is the backlog trend, as a fraction of the phase's
// declared rate, past which the phase counts against backlog_bounded.
const backlogSlopeFraction = 0.05

// headroomFraction is the share of its CPU cap a generator container may
// use before the sample counts against generator_headroom.
const headroomFraction = 0.8

// MeasureSummary is what every run measures, whatever it declares: latency
// from scheduled time over the whole run and per phase, and the committed
// produces per second. All of it is computed by the server over the record
// rows; nothing is merged from per-process numbers.
type MeasureSummary struct {
	Produce    datastore.LatencySummary     `json:"produce"`
	EndToEnd   datastore.LatencySummary     `json:"end_to_end"`
	Server     ServerSummary                `json:"server"`
	Phases     []PhaseSummary               `json:"phases"`
	Throughput []datastore.ThroughputSample `json:"throughput"`
}

// ServerSummary is what the server paid for the run: the observer's counter
// deltas over the producer window, and the WAL and transaction cost per
// committed message.
type ServerSummary struct {
	Deltas                 datastore.ServerDeltas `json:"deltas"`
	WalBytesPerMessage     float64                `json:"wal_bytes_per_message"`
	WalRecordsPerMessage   float64                `json:"wal_records_per_message"`
	WalFpiPerMessage       float64                `json:"wal_fpi_per_message"`
	TransactionsPerMessage float64                `json:"transactions_per_message"`
}

// PhaseSummary is one producer phase measured: the total rate it achieved
// against the total it declared across streams, the latency of the produces
// it scheduled, and the three guards read over its own window -- so a
// stepped ladder reads its sustainable rung off the phase rows even when
// the run-level guards are declared report.
type PhaseSummary struct {
	Name         string                   `json:"name"`
	DeclaredRate int                      `json:"declared_rate"`
	AchievedRate float64                  `json:"achieved_rate"`
	Produce      datastore.LatencySummary `json:"produce"`
	EndToEnd     datastore.LatencySummary `json:"end_to_end"`

	BacklogSlope     float64 `json:"backlog_slope"` // the steepest group's, messages per second
	ScheduleSlips    int64   `json:"schedule_slips"`
	HeadroomBreaches int64   `json:"headroom_breaches"`
	Held             bool    `json:"held"` // every guard inside its tolerance

	// median CPU of each service's containers over the phase, percent of
	// one core -- what names the limiter
	PostgresCpu float64 `json:"postgres_cpu"`
	ProducerCpu float64 `json:"producer_cpu"`
	ConsumerCpu float64 `json:"consumer_cpu"`
}

// measure reads the produce side -- produce latency, throughput, the
// server's counters, and each phase's guards -- from the phase rows'
// windows. End-to-end latency needs the handler records, which are whole
// only after the drain, so measureEndToEnd fills it afterwards.
func (c *Checker) measure(ctx context.Context, targets []datastore.Target, phases []record.PhaseRecord) (*MeasureSummary, error) {
	summary := &MeasureSummary{Phases: []PhaseSummary{}}
	for _, phase := range c.declared.Producer {
		from, to, err := phaseWindow(phases, phase)
		if err != nil {
			return nil, err
		}
		measured, err := c.measurePhase(ctx, targets, phase, from, to)
		if err != nil {
			return nil, err
		}
		summary.Phases = append(summary.Phases, measured)
	}

	runStart, runEnd, err := runWindow(phases, c.declared.Producer)
	if err != nil {
		return nil, err
	}
	summary.Produce, err = c.ds.ReadProduceLatency(ctx, runStart, runEnd)
	if err != nil {
		return nil, err
	}
	summary.Throughput, err = c.ds.ReadThroughput(ctx, runStart, runEnd)
	if err != nil {
		return nil, err
	}
	summary.Server, err = c.measureServer(ctx, runStart, runEnd, summary.Produce.Count)
	if err != nil {
		return nil, err
	}
	return summary, nil
}

// measureEndToEnd fills the end-to-end latency of every phase and the whole
// run once the handler records are loaded.
func (c *Checker) measureEndToEnd(ctx context.Context, summary *MeasureSummary, phases []record.PhaseRecord) error {
	for i, phase := range c.declared.Producer {
		from, to, err := phaseWindow(phases, phase)
		if err != nil {
			return err
		}
		summary.Phases[i].EndToEnd, err = c.ds.ReadEndToEndLatency(ctx, from, to)
		if err != nil {
			return err
		}
	}
	runStart, runEnd, err := runWindow(phases, c.declared.Producer)
	if err != nil {
		return err
	}
	summary.EndToEnd, err = c.ds.ReadEndToEndLatency(ctx, runStart, runEnd)
	return err
}

func (c *Checker) measureServer(ctx context.Context, from time.Time, to time.Time, committed int64) (ServerSummary, error) {
	deltas, err := c.ds.ReadServerDeltas(ctx, from, to)
	if err != nil {
		return ServerSummary{}, err
	}
	perMessage := 1 / float64(committed)
	return ServerSummary{
		Deltas:                 deltas,
		WalBytesPerMessage:     float64(deltas.WalBytes) * perMessage,
		WalRecordsPerMessage:   float64(deltas.WalRecords) * perMessage,
		WalFpiPerMessage:       float64(deltas.WalFpi) * perMessage,
		TransactionsPerMessage: float64(deltas.XactCommit) * perMessage,
	}, nil
}

// countScheduleSlips is the schedule_kept check over the producer window.
func (c *Checker) countScheduleSlips(ctx context.Context, phases []record.PhaseRecord) (datastore.Measurement, error) {
	from, to, err := runWindow(phases, c.declared.Producer)
	if err != nil {
		return datastore.Measurement{}, err
	}
	return c.ds.CountScheduleSlips(ctx, from, to, scheduleTolerance)
}

// countDivergingPhases is the backlog_bounded check: producer phases in
// which any group's backlog slope exceeds backlogSlopeFraction of the
// phase's per-stream rate.
func (c *Checker) countDivergingPhases(ctx context.Context, targets []datastore.Target, phases []record.PhaseRecord) (datastore.Measurement, error) {
	measured := datastore.Measurement{ExampleOf: examplePhase, Examples: []string{}}
	for _, phase := range c.declared.Producer {
		from, to, err := phaseWindow(phases, phase)
		if err != nil {
			return datastore.Measurement{}, err
		}
		for _, target := range targets {
			slope, err := c.ds.ReadBacklogSlope(ctx, from, to, target.Stream, target.Group)
			if err != nil {
				return datastore.Measurement{}, err
			}
			if slope > backlogSlopeFraction*float64(phase.Rate) {
				measured.Count++
				if len(measured.Examples) < exampleLimit {
					measured.Examples = append(measured.Examples, fmt.Sprintf("%s %s/%s +%.1f/s", phase.Name, target.Stream, target.Group, slope))
				}
			}
		}
	}
	return measured, nil
}

// countHeadroomBreaches is the generator_headroom check over the producer
// window; a container compose left uncapped is judged against the engine's
// CPU count from the fingerprint.
func (c *Checker) countHeadroomBreaches(ctx context.Context, phases []record.PhaseRecord) (datastore.Measurement, error) {
	from, to, err := runWindow(phases, c.declared.Producer)
	if err != nil {
		return datastore.Measurement{}, err
	}
	return c.ds.CountHeadroomBreaches(ctx, from, to, headroomFraction, float64(c.fingerprint.Docker.Cpus))
}

func (c *Checker) measurePhase(ctx context.Context, targets []datastore.Target, phase scenario.ProducerPhase, from time.Time, to time.Time) (PhaseSummary, error) {
	measured := PhaseSummary{Name: phase.Name, DeclaredRate: phase.Rate * len(c.declared.Streams)}
	var err error
	measured.Produce, err = c.ds.ReadProduceLatency(ctx, from, to)
	if err != nil {
		return PhaseSummary{}, err
	}
	measured.AchievedRate = float64(measured.Produce.Count) / to.Sub(from).Seconds()

	for _, target := range targets {
		slope, err := c.ds.ReadBacklogSlope(ctx, from, to, target.Stream, target.Group)
		if err != nil {
			return PhaseSummary{}, err
		}
		measured.BacklogSlope = max(measured.BacklogSlope, slope)
	}
	slips, err := c.ds.CountScheduleSlips(ctx, from, to, scheduleTolerance)
	if err != nil {
		return PhaseSummary{}, err
	}
	measured.ScheduleSlips = slips.Count
	breaches, err := c.ds.CountHeadroomBreaches(ctx, from, to, headroomFraction, float64(c.fingerprint.Docker.Cpus))
	if err != nil {
		return PhaseSummary{}, err
	}
	measured.HeadroomBreaches = breaches.Count
	measured.Held = measured.BacklogSlope <= backlogSlopeFraction*float64(phase.Rate) && slips.Count == 0 && breaches.Count == 0

	measured.PostgresCpu, err = c.ds.ReadContainerCpu(ctx, from, to, "postgres")
	if err != nil {
		return PhaseSummary{}, err
	}
	measured.ProducerCpu, err = c.ds.ReadContainerCpu(ctx, from, to, "producer")
	if err != nil {
		return PhaseSummary{}, err
	}
	measured.ConsumerCpu, err = c.ds.ReadContainerCpu(ctx, from, to, "consumer")
	if err != nil {
		return PhaseSummary{}, err
	}
	return measured, nil
}

// ***************
// *** HELPERS ***
// ***************

// examplePhase labels a check whose examples are phase names.
const examplePhase = "phase"

// exampleLimit bounds a check's examples, as in the datastore.
const exampleLimit = 5

// runWindow is [first phase started, last phase ended] across the producer
// phases.
func runWindow(phases []record.PhaseRecord, declared []scenario.ProducerPhase) (time.Time, time.Time, error) {
	var runStart, runEnd time.Time
	for _, phase := range declared {
		from, to, err := phaseWindow(phases, phase)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		if runStart.IsZero() || from.Before(runStart) {
			runStart = from
		}
		if to.After(runEnd) {
			runEnd = to
		}
	}
	return runStart, runEnd, nil
}

// phaseWindow is [started, ended) of one producer phase from its run_phase
// rows: the earliest start and latest end across the streams' rows. A phase
// missing either row was cut short, and the run cannot be measured.
func phaseWindow(phases []record.PhaseRecord, phase scenario.ProducerPhase) (time.Time, time.Time, error) {
	var started, ended time.Time
	for _, row := range phases {
		if row.Kind != record.PhaseKindProducer || row.Name != phase.Name {
			continue
		}
		switch row.Status {
		case record.PhaseStatusStarted:
			if started.IsZero() || row.At.Before(started) {
				started = row.At
			}
		case record.PhaseStatusEnded:
			if row.At.After(ended) {
				ended = row.At
			}
		}
	}
	if started.IsZero() || ended.IsZero() {
		return time.Time{}, time.Time{}, fmt.Errorf("phase %q has no started and ended rows -- the producer did not finish it", phase.Name)
	}
	return started, ended, nil
}
