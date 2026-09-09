package admin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/agentstax/sqlstreams/pkg/alert"
	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/migrate"
	"github.com/agentstax/sqlstreams/pkg/schedule"
	"github.com/agentstax/sqlstreams/pkg/stream"
)

// GetAlertSnapshot evaluates a built-in using its current schedule declaration.
// It writes nothing and returns schedule.ErrScheduleNotFound if the declaration is absent.
func (a *MessageAdmin) GetAlertSnapshot(ctx context.Context, name string, owner *common.Owner) (*alert.AlertEvaluationSnapshot, error) {
	evaluator, found := a.alertEvaluators[name]
	if !found {
		return nil, fmt.Errorf("unrecognized built-in alert: %q", name)
	}

	declared, err := a.scheduleController.Get(ctx, "alert."+name)
	if err != nil {
		return nil, err
	}
	if declared == nil {
		return nil, schedule.ErrScheduleNotFound.With("schedule", "alert."+name)
	}
	var policy *alert.JobPayload
	if declared.SchemaVersion != common.SchemaVersionOf[alert.JobPayload]() {
		return nil, fmt.Errorf("unrecognized alert policy schema version: %d", declared.SchemaVersion)
	}
	if err := json.Unmarshal(declared.Payload, &policy); err != nil {
		return nil, err
	}
	return evaluator.Evaluate(ctx, owner, policy)
}

// ListAlerts returns the current head per (alert, owner) on __system.alerts --
// each key's latest publish, active or resolved, within the stream's retention
// window.
// Returns migrate.ErrNotRegistered until RegisterSystem has run.
func (a *MessageAdmin) ListAlerts(ctx context.Context) ([]*common.StoredMessage[alert.Alert], error) {
	found, err := a.alertsStream(ctx)
	if err != nil {
		return nil, err
	}
	return a.heads.ListHeads[alert.Alert](ctx, found.Id)
}

// GetAlert returns one (alert, owner) key's current retained alert, or nil
// if no retained alert has its message key.
// Returns migrate.ErrNotRegistered until RegisterSystem has run.
func (a *MessageAdmin) GetAlert(ctx context.Context, messageKey string) (*common.StoredMessage[alert.Alert], error) {
	found, err := a.alertsStream(ctx)
	if err != nil {
		return nil, err
	}
	return a.heads.GetHead[alert.Alert](ctx, found.Id, messageKey)
}

// ListAlertMessages returns one (alert, owner) key's retained alerts, newest
// first. messageKey is alert.MessageKey(name, owner); limit is required.
// Returns migrate.ErrNotRegistered until RegisterSystem has run.
func (a *MessageAdmin) ListAlertMessages(ctx context.Context, messageKey string, limit int) ([]*common.StoredMessage[alert.Alert], error) {
	found, err := a.alertsStream(ctx)
	if err != nil {
		return nil, err
	}
	return a.heads.ListKeyMessages[alert.Alert](ctx, found.Id, messageKey, limit)
}

func (a *MessageAdmin) alertsStream(ctx context.Context) (*stream.Stream, error) {
	found, err := a.streamController.Get(ctx, alert.AlertStreamName)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, migrate.ErrNotRegistered.With("stream", alert.AlertStreamName)
	}
	return found, nil
}
