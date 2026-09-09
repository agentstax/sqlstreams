package scheduler

import (
	"fmt"

	"github.com/agentstax/sqlstreams/pkg/common"
)

// ScheduleRunOptions controls one immediate run of a registered schedule.
// Every field is optional.
type ScheduleRunOptions struct {
	// Concurrency - the produced request's concurrent-run policy.
	// Default: parallel (the request runs even while a previous one is still running).
	//
	// Set exclusive to run early WITHOUT overlapping a request already running.
	Concurrency common.ConcurrencyPolicy
}

func (o *ScheduleRunOptions) WithDefaults() *ScheduleRunOptions {
	if o.Concurrency == "" {
		o.Concurrency = common.ConcurrencyParallel
	}
	return o
}

func (o *ScheduleRunOptions) Validate() error {
	if err := o.Concurrency.Validate(); err != nil {
		return fmt.Errorf("Concurrency: %w", err)
	}
	return nil
}
