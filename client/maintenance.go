package sqlstreams

import "context"

// MaintenanceHandle operates one stream's janitor or vacuum worker.
// Settings are declared through StreamConfig; operations preserve those settings.
type MaintenanceHandle struct {
	streamName string
	workerName string
	client     *Client
}

// Janitor selects this stream's retention cleanup. No I/O until an operation runs.
// New streams start with janitor active.
func (t *StreamHandle[Message]) Janitor() *MaintenanceHandle {
	return &MaintenanceHandle{streamName: t.name, workerName: "stream_janitor", client: t.client}
}

// Vacuum selects this stream's key-table vacuum. No I/O until an operation runs.
// New streams start with vacuum suspended.
func (t *StreamHandle[Message]) Vacuum() *MaintenanceHandle {
	return &MaintenanceHandle{streamName: t.name, workerName: "stream_vacuum", client: t.client}
}

// Suspend prevents new claims and requests running maintenance to stop at its
// next successful heartbeat. It returns without waiting for work to stop.
// Returns ErrStreamNotFound or ErrWorkerNotFound when the resource is absent.
func (h *MaintenanceHandle) Suspend(ctx context.Context) error {
	return h.client.admin.SetWorkerTarget(ctx, h.streamName, h.workerName, 0)
}

// Unsuspend permits one maintenance instance to run.
// A manager must be running to claim it.
// Returns ErrStreamNotFound or ErrWorkerNotFound when the resource is absent.
func (h *MaintenanceHandle) Unsuspend(ctx context.Context) error {
	return h.client.admin.SetWorkerTarget(ctx, h.streamName, h.workerName, 1)
}

// Status reports suspension, claims, and consecutive failures.
// Returns ErrStreamNotFound or ErrWorkerNotFound when the resource is absent.
func (h *MaintenanceHandle) Status(ctx context.Context) (*WorkerSnapshot, error) {
	return h.client.admin.WorkerStatus(ctx, h.streamName, h.workerName)
}
