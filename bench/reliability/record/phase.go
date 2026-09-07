package record

import "time"

type PhaseKind string

const (
	PhaseProducer  PhaseKind = "producer"
	PhaseConsumers PhaseKind = "consumers"
)

type PhaseStatus string

const (
	PhaseStarted PhaseStatus = "started"
	PhaseEnded   PhaseStatus = "ended"
)

// Phase is one row of run_phase: a timeline edge the checker uses to
// attribute what it counts to the window it happened in. Detail is the
// phase's own line from the scenario ("steady 200/s 10m", "consumers 3").
type Phase struct {
	At     time.Time   `json:"at"`
	Role   string      `json:"role"`
	Kind   PhaseKind   `json:"kind"`
	Name   string      `json:"name"`
	Status PhaseStatus `json:"status"`
	Detail string      `json:"detail"`
}
