package scheduler

import (
	"context"
	"errors"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/datastore"
	"github.com/allegedlyreliable/sqlstreams/pkg/schedule"
	"github.com/allegedlyreliable/sqlstreams/pkg/systemmanager"
)

// SchedulerInstance is a registered schedule: Schedule keeps the system producing it.
type SchedulerInstance[Message common.Versioned] struct {
	Registered *schedule.Schedule // the schedule row as Register left it
	Payload    *Message           // the stored payload every run produces
	Config     *SchedulerConfig   // the resolved config Register stored

	ds *datastore.PostgresDatastore
}

// cfg arrives already resolved by Register.
func newSchedulerInstance[Message common.Versioned](registered *schedule.Schedule, payload *Message, ds *datastore.PostgresDatastore, cfg *SchedulerConfig) (*SchedulerInstance[Message], error) {
	if registered == nil {
		return nil, errors.New("schedule must not be nil")
	}
	if payload == nil {
		return nil, errors.New("payload must not be nil")
	}
	if ds == nil {
		return nil, errors.New("datastore must not be nil")
	}
	if cfg == nil {
		return nil, errors.New("config must not be nil")
	}
	return &SchedulerInstance[Message]{
		Registered: registered,
		Payload:    payload,
		Config:     cfg,
		ds:         ds,
	}, nil
}

// Schedule runs the system manager until ctx cancels -- what `sqlstreams manager
// run` does; the schedule producer worker produces every registered
// schedule, not just this one. A requested stop returns nil.
func (i *SchedulerInstance[Message]) Schedule(ctx context.Context) error {
	systemManager, err := systemmanager.NewSystemManager(i.ds, nil)
	if err != nil {
		return err
	}
	return systemManager.Run(ctx)
}
