package vulkan

import (
	"context"

	"github.com/agentstax/vulkan/pkg/alert"
	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/common/diagnostic"
)

// TopicAlertsHandle names one topic's alerts resource, holding no database
// row.
type TopicAlertsHandle struct {
	topicName string
	client    *Client
}

// Alerts returns the topic's alerts handle. It performs no I/O.
func (t *TopicHandle[Message]) Alerts() *TopicAlertsHandle {
	return &TopicAlertsHandle{topicName: t.name, client: t.client}
}

// Definitions returns the topic-scoped Vulkan alert definitions ordered by
// VK code. It performs no I/O.
func (t *TopicAlertsHandle) Definitions() []AlertDefinition {
	return alert.Definitions(diagnostic.MetricScopeTopic)
}

// Latest returns the current alert per name owned by the topic, active or
// resolved, ordered by message key. Returns ErrTopicNotFound when the topic
// is not registered.
func (t *TopicAlertsHandle) Latest(ctx context.Context) ([]*Alert, error) {
	owner, err := t.client.admin.TopicOwner(ctx, t.topicName)
	if err != nil {
		return nil, err
	}

	stored, err := t.client.admin.ListAlerts(ctx)
	if err != nil {
		return nil, err
	}
	alerts := make([]*Alert, 0, len(stored))
	for _, head := range stored {
		if head.Message.Owner.Kind() == common.OwnerTopic && head.Message.Owner.TopicId == owner.TopicId {
			alerts = append(alerts, head.Message)
		}
	}
	return alerts, nil
}

// Alert names one alert owned by the topic. It performs no I/O. Built-in
// selectors are preferred when one is available.
func (t *TopicAlertsHandle) Alert(name string) *AlertHandle {
	return newAlertHandle(t.client, name, t.topicName, "")
}

// PartitionCount selects the topic's partition_count alert.
func (t *TopicAlertsHandle) PartitionCount() *AlertHandle {
	return t.Alert(alert.AlertPartitionCount.Name)
}

// CompactionReadCost selects the topic's compaction_read_cost alert.
func (t *TopicAlertsHandle) CompactionReadCost() *AlertHandle {
	return t.Alert(alert.AlertCompactionReadCost.Name)
}

// WorkerLiveness selects the topic's worker_liveness alert.
func (t *TopicAlertsHandle) WorkerLiveness() *AlertHandle {
	return t.Alert(alert.AlertWorkerLiveness.Name)
}
