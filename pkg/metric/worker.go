package metric

import (
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
)

// WorkerStatus describes suspension, live claims, and recorded failures.
type WorkerStatus string

const (
	WorkerSuspended WorkerStatus = "suspended" // target_instances = 0
	WorkerClaimed   WorkerStatus = "claimed"   // live instances without a recorded failure streak
	WorkerFailing   WorkerStatus = "failing"   // live instances report consecutive failures
	WorkerUnclaimed WorkerStatus = "unclaimed" // no live instance row and not suspended
)

// WorkerSnapshot reports one worker's operational target, live instances, and recorded failures.
type WorkerSnapshot struct {
	Owner  *common.Owner `json:"owner"`  // system, stream, or consumer group that owns the worker
	Name   string        `json:"worker"` // declared worker name
	Status WorkerStatus  `json:"status"` // suspension takes precedence over live claims and failures

	TargetInstances int `json:"target_instances"` // zero suspends; -1 permits an unlimited number of instances
	LiveInstances   int `json:"live_instances"`   // claims whose expiry is still in the future

	Attempts     int           `json:"attempts"`      // largest consecutive-failure streak across live instances
	UnclaimedFor time.Duration `json:"unclaimed_for"` // now() - the newest expires_at while unclaimed; zero if live or expired rows were deleted
}
