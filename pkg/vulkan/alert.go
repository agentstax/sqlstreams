package vulkan

import (
	"context"
	"fmt"

	"github.com/agentstax/vulkan/pkg/alert"
	"github.com/agentstax/vulkan/pkg/common"
)

// AlertHandle names one alert on one owner, holding no database row.
type AlertHandle struct {
	name      string
	topicName string // "" for a system-owned alert
	groupName string // "" unless the owner is a consumer group
	client    *Client
}

func newAlertHandle(client *Client, name string, topicName string, groupName string) *AlertHandle {
	return &AlertHandle{name: name, topicName: topicName, groupName: groupName, client: client}
}

// Snapshot evaluates a built-in alert using retained evidence and the current
// schedule policy, without writes. Queued checks retain their consumed policy.
// Returns ErrTopicNotFound or ErrConsumerNotFound for a missing owner and
// ErrScheduleNotFound for a missing schedule; unsupported names/scopes return errors.
func (a *AlertHandle) Snapshot(ctx context.Context) (*AlertEvaluationSnapshot, error) {
	owner, err := a.owner(ctx)
	if err != nil {
		return nil, err
	}
	return a.client.admin.GetAlertSnapshot(ctx, a.name, owner)
}

// Latest returns the current alert, active or resolved, or nil if no
// retained alert has its key.
func (a *AlertHandle) Latest(ctx context.Context) (*Alert, error) {
	messageKey, err := a.messageKey(ctx)
	if err != nil {
		return nil, err
	}

	stored, err := a.client.admin.GetAlert(ctx, messageKey)
	if err != nil {
		return nil, err
	}
	if stored == nil {
		return nil, nil
	}
	return stored.Message, nil
}

// History returns the alert's retained messages newest first. limit must be
// positive.
func (a *AlertHandle) History(ctx context.Context, limit int) ([]*Alert, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be > 0, got %d", limit)
	}

	messageKey, err := a.messageKey(ctx)
	if err != nil {
		return nil, err
	}

	stored, err := a.client.admin.ListAlertMessages(ctx, messageKey, limit)
	if err != nil {
		return nil, err
	}
	return unwrapMessages(stored), nil
}

// messageKey resolves the owner the handle's names address and hands it
// to the one composer.
func (a *AlertHandle) messageKey(ctx context.Context) (string, error) {
	owner, err := a.owner(ctx)
	if err != nil {
		return "", err
	}
	return alert.MessageKey(a.name, owner)
}

func (a *AlertHandle) owner(ctx context.Context) (*common.Owner, error) {
	switch {
	case a.groupName != "":
		return a.client.admin.ConsumerGroupOwner(ctx, a.topicName, a.groupName)
	case a.topicName != "":
		return a.client.admin.TopicOwner(ctx, a.topicName)
	default:
		return a.client.admin.SystemOwner(ctx)
	}
}
