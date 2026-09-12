package sqlstreams

import (
	"context"
	"errors"

	"github.com/allegedlyreliable/sqlstreams/pkg/scheduler"
)

// SchedulerInstance is a registered schedule. Schedule runs the system
// manager until ctx cancels; the manager's schedule producer produces every
// registered schedule, not only this one.
type SchedulerInstance[Message Versioned] struct {
	client   *Client
	instance *scheduler.SchedulerInstance[Message]
}

func newSchedulerInstance[Message Versioned](client *Client, instance *scheduler.SchedulerInstance[Message]) (*SchedulerInstance[Message], error) {
	if client == nil {
		return nil, errors.New("client must not be nil")
	}
	if instance == nil {
		return nil, errors.New("instance must not be nil")
	}
	return &SchedulerInstance[Message]{
		client:   client,
		instance: instance,
	}, nil
}

// Schedule blocks running the system manager, which produces every
// registered schedule; cancel ctx to stop and return nil.
func (s *SchedulerInstance[Message]) Schedule(ctx context.Context) error {
	return s.client.manager.Run(ctx)
}
