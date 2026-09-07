package worker

import "time"

// WorkerInstanceSnapshot is a retained instance's lease interval.
type WorkerInstanceSnapshot struct {
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// WorkerInstanceHistory contains lease snapshots ordered by creation time descending.
type WorkerInstanceHistory struct {
	EvaluatedAt time.Time                `json:"evaluated_at"`
	Instances   []WorkerInstanceSnapshot `json:"instances"`
}
