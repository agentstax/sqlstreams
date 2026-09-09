package controller

import (
	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/worker"
	"github.com/agentstax/sqlstreams/pkg/worker/controller/datastore"
	"time"
	"uuid"
)

func toWorkerInstanceHistory(current time.Time, rows []datastore.WorkerInstanceSnapshotRow) *worker.WorkerInstanceHistory {
	instances := make([]worker.WorkerInstanceSnapshot, 0, len(rows))
	for _, row := range rows {
		instances = append(instances, worker.WorkerInstanceSnapshot{CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt})
	}
	return &worker.WorkerInstanceHistory{EvaluatedAt: current, Instances: instances}
}

func toWorker(data datastore.ListWorkersRow) (*worker.Worker, error) {
	var owner *common.Owner
	var err error
	switch {
	case data.ConsumerGroupId != nil:
		owner, err = common.NewConsumerGroupOwner(data.OwnerSystemId, data.OwnerStreamId, *data.ConsumerGroupId, data.ConsumerGroup)
	case data.StreamId != nil:
		owner, err = common.NewStreamOwner(data.OwnerSystemId, *data.StreamId, data.StreamName)
	default:
		owner, err = common.NewSystemOwner(data.OwnerSystemId)
	}
	if err != nil {
		return nil, err
	}

	return &worker.Worker{
		Id:              data.Id,
		Name:            data.Name,
		Owner:           owner,
		Metadata:        data.Metadata,
		TargetInstances: worker.InstanceTarget(data.TargetInstances),
	}, nil
}

// the owner was the lookup key here, so unlike toWorker there are no join
// columns to resolve it from
func toOwnedWorker(data *datastore.WorkerConfigRow, owner *common.Owner) *worker.Worker {
	return &worker.Worker{
		Id:              data.Id,
		Name:            data.Name,
		Owner:           owner,
		Metadata:        data.Metadata,
		TargetInstances: worker.InstanceTarget(data.TargetInstances),
	}
}

func toWorkerInstance(data *datastore.WorkerInstanceRow) *worker.WorkerInstance {
	return &worker.WorkerInstance{
		Id:       data.Id,
		WorkerId: data.WorkerId,
		Token:    uuid.UUID(data.Token.Bytes),
		Attempts: data.Attempts,
	}
}
