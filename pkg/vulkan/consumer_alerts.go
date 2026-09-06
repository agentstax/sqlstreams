package vulkan

import (
	"context"

	"github.com/agentstax/vulkan/pkg/alert"
	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/common/diagnostic"
)

// ConsumerAlertsHandle names one consumer group's alerts resource, holding no
// database row.
type ConsumerAlertsHandle struct {
	topicName string
	groupName string
	client    *Client
}

// Alerts returns the consumer group's alerts handle. It performs no I/O.
func (h *ConsumerHandle[Message]) Alerts() *ConsumerAlertsHandle {
	return &ConsumerAlertsHandle{topicName: h.topicName, groupName: h.name, client: h.client}
}

// Definitions returns the consumer-group-scoped Vulkan alert definitions
// ordered by VK code. It performs no I/O.
func (h *ConsumerAlertsHandle) Definitions() []AlertDefinition {
	return alert.Definitions(diagnostic.MetricScopeConsumerGroup)
}

// Latest returns the current alert per name owned by the consumer group,
// active or resolved, ordered by message key. Returns ErrTopicNotFound or
// ErrConsumerNotFound when either side is missing.
func (h *ConsumerAlertsHandle) Latest(ctx context.Context) ([]*Alert, error) {
	owner, err := h.client.admin.ConsumerGroupOwner(ctx, h.topicName, h.groupName)
	if err != nil {
		return nil, err
	}

	stored, err := h.client.admin.ListAlerts(ctx)
	if err != nil {
		return nil, err
	}
	alerts := make([]*Alert, 0, len(stored))
	for _, head := range stored {
		if head.Message.Owner.Kind() == common.OwnerConsumerGroup && head.Message.Owner.ConsumerGroupId == owner.ConsumerGroupId {
			alerts = append(alerts, head.Message)
		}
	}
	return alerts, nil
}

// Alert names one alert owned by the consumer group. It performs no I/O. No
// built-in is group-owned today, so this is the handle's only read.
func (h *ConsumerAlertsHandle) Alert(name string) *AlertHandle {
	return newAlertHandle(h.client, name, h.topicName, h.groupName)
}
