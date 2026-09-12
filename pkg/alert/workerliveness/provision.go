package workerliveness

import (
	"context"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/consume"
	"github.com/allegedlyreliable/sqlstreams/pkg/migrate"
	"github.com/allegedlyreliable/sqlstreams/pkg/schedule"
	"github.com/allegedlyreliable/sqlstreams/pkg/worker"
	workercontroller "github.com/allegedlyreliable/sqlstreams/pkg/worker/controller"
)

// Declare creates the alert's consumer group on the schedules stream and its
// job-name binding declaration, then writes the alert's config onto the group's
// worker row -- the newest declaration wins. RegisterSystem runs it every time.
func (p *WorkerLivenessProvisioner) Declare(ctx context.Context, owner *common.Owner) error {
	if err := workercontroller.ValidateOwner(owner, common.OwnerSystem, JobName); err != nil {
		return err
	}

	jobRequestsStream, err := p.streams.Get(ctx, schedule.ScheduleStreamName)
	if err != nil {
		return err
	}
	if jobRequestsStream == nil {
		return migrate.ErrNotRegistered.With("stream", schedule.ScheduleStreamName)
	}

	group, err := p.consumers.RegisterGroup(ctx, jobRequestsStream.Id, JobName, consume.Beginning())
	if err != nil {
		return err
	}

	// a waiting outcome is fine -- the consumer retries the declaration in Consume
	if _, err := p.consumers.DeclareBindings(ctx, jobRequestsStream.Id, group.Id, []string{JobName}, time.Now()); err != nil {
		return err
	}

	groupOwner, err := common.NewConsumerGroupOwner(jobRequestsStream.SystemId, jobRequestsStream.Id, group.Id, group.Name)
	if err != nil {
		return err
	}
	return p.workers.DeclareWorker(ctx, p.definition, groupOwner)
}

// Provision claims one live instance. nil = declined (target_instances
// already filled) -- not an error, try again later.
func (p *WorkerLivenessProvisioner) Provision(ctx context.Context, declared *worker.Worker) (worker.Execution, error) {
	parsed, err := workercontroller.ParseMetadata[workerLivenessMetadata](declared.Metadata)
	if err != nil {
		return nil, err
	}
	if err := parsed.Validate(); err != nil {
		return nil, err
	}
	claimed, err := p.workers.RegisterInstance(ctx, declared.Id, declared.Owner, common.OwnerConsumerGroup, JobName, p.Config.InstanceTTL)
	if err != nil || claimed == nil {
		return nil, err
	}
	return newWorkerLivenessInstance(p, declared.Owner, claimed, parsed.RepeatInterval)
}
