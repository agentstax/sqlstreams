package admin

import (
	"context"

	"github.com/agentstax/vulkan/pkg/alert"
	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/migrate"
	"github.com/agentstax/vulkan/pkg/topic"
)

// ListAlerts returns the current head per (alert, owner) on __system.alerts --
// each key's latest publish, active or resolved, within the topic's retention
// window.
// Returns migrate.ErrNotRegistered until RegisterSystem has run.
func (a *MessageAdmin) ListAlerts(ctx context.Context) ([]*common.StoredMessage[alert.Alert], error) {
	found, err := a.alertsTopic(ctx)
	if err != nil {
		return nil, err
	}
	return a.heads.ListHeads[alert.Alert](ctx, found.Id)
}

// GetAlert returns one (alert, owner) key's current retained alert, or nil
// if no retained alert has its message key.
// Returns migrate.ErrNotRegistered until RegisterSystem has run.
func (a *MessageAdmin) GetAlert(ctx context.Context, messageKey string) (*common.StoredMessage[alert.Alert], error) {
	found, err := a.alertsTopic(ctx)
	if err != nil {
		return nil, err
	}
	return a.heads.GetHead[alert.Alert](ctx, found.Id, messageKey)
}

// ListAlertMessages returns one (alert, owner) key's retained alerts, newest
// first. messageKey is alert.MessageKey(name, owner); limit is required.
// Returns migrate.ErrNotRegistered until RegisterSystem has run.
func (a *MessageAdmin) ListAlertMessages(ctx context.Context, messageKey string, limit int) ([]*common.StoredMessage[alert.Alert], error) {
	found, err := a.alertsTopic(ctx)
	if err != nil {
		return nil, err
	}
	return a.heads.ListKeyMessages[alert.Alert](ctx, found.Id, messageKey, limit)
}

func (a *MessageAdmin) alertsTopic(ctx context.Context) (*topic.Topic, error) {
	found, err := a.topicController.Get(ctx, alert.AlertTopicName)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, migrate.ErrNotRegistered.With("topic", alert.AlertTopicName)
	}
	return found, nil
}
