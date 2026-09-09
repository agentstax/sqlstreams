package controller

import (
	"context"

	"github.com/agentstax/sqlstreams/pkg/metric"
)

// WorkerSnapshots is every worker row's current claim state.
func (c *MetricController) WorkerSnapshots(ctx context.Context) ([]metric.WorkerSnapshot, error) {
	data, err := c.datastore.WorkerSnapshots(ctx)
	if err != nil {
		return nil, err
	}

	workers := make([]metric.WorkerSnapshot, 0, len(data))
	for _, row := range data {
		snapshot, err := toWorkerSnapshot(row)
		if err != nil {
			return nil, err
		}
		workers = append(workers, snapshot)
	}
	return workers, nil
}
