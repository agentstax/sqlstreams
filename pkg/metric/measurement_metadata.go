package metric

import (
	"errors"

	"github.com/agentstax/sqlstreams/pkg/common"
)

// PartitionMeasurementMetadata records applicability alongside the partition count.
type PartitionMeasurementMetadata struct {
	CompactionStatus string `json:"compaction_status"`
}

func NewPartitionMeasurementMetadata(compacted bool) (*PartitionMeasurementMetadata, error) {
	status := "uncompacted"
	if compacted {
		status = "compacted"
	}
	return &PartitionMeasurementMetadata{CompactionStatus: status}, nil
}

// UnclaimedWorkerMetadata identifies a worker in a retained liveness measurement.
type UnclaimedWorkerMetadata struct {
	Name            string        `json:"worker"`
	Owner           *common.Owner `json:"owner"`
	TargetInstances int           `json:"target_instances"`
}

func NewUnclaimedWorkerMetadata(name string, owner *common.Owner, targetInstances int) (*UnclaimedWorkerMetadata, error) {
	if name == "" {
		return nil, errors.New("name must not be empty")
	}
	if owner == nil {
		return nil, errors.New("owner must not be nil")
	}
	return &UnclaimedWorkerMetadata{Name: name, Owner: owner, TargetInstances: targetInstances}, nil
}

// WorkerMeasurementMetadata describes the workers counted by the same measurement.
type WorkerMeasurementMetadata struct {
	Workers []*UnclaimedWorkerMetadata `json:"workers"`
}

func NewWorkerMeasurementMetadata(workers []*UnclaimedWorkerMetadata) (*WorkerMeasurementMetadata, error) {
	return &WorkerMeasurementMetadata{Workers: workers}, nil
}
