package admin

import (
	"context"

	"github.com/allegedlyreliable/sqlstreams/pkg/metric"
	"github.com/allegedlyreliable/sqlstreams/pkg/worker"
)

func (a *MessageAdmin) SetWorkerTarget(ctx context.Context, streamName string, workerName string, target worker.InstanceTarget) error {
	owner, err := a.StreamOwner(ctx, streamName)
	if err != nil {
		return err
	}
	current, err := a.workerController.GetWorker(ctx, workerName, owner)
	if err != nil {
		return err
	}
	if current == nil {
		return worker.ErrWorkerNotFound.With("stream", streamName, "worker", workerName)
	}
	return a.workerController.UpdateTargetInstances(ctx, current.Id, target)
}

func (a *MessageAdmin) WorkerStatus(ctx context.Context, streamName string, workerName string) (*metric.WorkerSnapshot, error) {
	owner, err := a.StreamOwner(ctx, streamName)
	if err != nil {
		return nil, err
	}
	snapshots, err := a.metricController.WorkerSnapshots(ctx)
	if err != nil {
		return nil, err
	}
	for _, snapshot := range snapshots {
		if snapshot.Owner.StreamId == owner.StreamId && snapshot.Owner.ConsumerGroupId == 0 && snapshot.Name == workerName {
			return &snapshot, nil
		}
	}
	return nil, worker.ErrWorkerNotFound.With("stream", streamName, "worker", workerName)
}
