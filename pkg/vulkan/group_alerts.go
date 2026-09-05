package vulkan

import (
	"context"

	"github.com/agentstax/vulkan/pkg/alert"
	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/common/diagnostic"
)

// GroupAlertsHandle names one consumer group's alerts resource, holding no
// database row.
type GroupAlertsHandle struct {
	topicName string
	groupName string
	client    *Client
}

// Alerts returns the consumer group's alerts handle. It performs no I/O.
func (g *GroupHandle[Message]) Alerts() *GroupAlertsHandle {
	return &GroupAlertsHandle{topicName: g.topicName, groupName: g.name, client: g.client}
}

// Definitions returns the consumer-group-scoped Vulkan alert definitions
// ordered by VK code. It performs no I/O.
func (g *GroupAlertsHandle) Definitions() []AlertDefinition {
	return alert.Definitions(diagnostic.MetricScopeConsumerGroup)
}

// Latest returns the current alert per name owned by the consumer group,
// active or resolved, ordered by message key. Returns ErrTopicNotFound or
// ErrGroupNotFound when either side is missing.
func (g *GroupAlertsHandle) Latest(ctx context.Context) ([]*Alert, error) {
	owner, err := g.client.admin.GroupOwner(ctx, g.topicName, g.groupName)
	if err != nil {
		return nil, err
	}

	stored, err := g.client.admin.ListAlerts(ctx)
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
func (g *GroupAlertsHandle) Alert(name string) *AlertHandle {
	return newAlertHandle(g.client, name, g.topicName, g.groupName)
}
