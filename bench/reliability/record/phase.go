package record

import "time"

type PhaseKind string

const (
	PhaseKindProducer  PhaseKind = "producer"
	PhaseKindConsumers PhaseKind = "consumers"
)

type PhaseStatus string

const (
	PhaseStatusStarted PhaseStatus = "started"
	PhaseStatusEnded   PhaseStatus = "ended"
)

// PhaseRecord is one row of run_phase: a timeline edge the checker uses to
// attribute what it counts to the window it happened in. Process is the
// name of the process that wrote it; Detail is the phase's own line from
// the scenario ("steady 200/s 10m", "consumers 3").
type PhaseRecord struct {
	At      time.Time   `json:"at"`
	Process string      `json:"process"`
	Kind    PhaseKind   `json:"kind"`
	Name    string      `json:"name"`
	Status  PhaseStatus `json:"status"`
	Detail  string      `json:"detail"`
}
