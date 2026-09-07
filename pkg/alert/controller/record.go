package controller

import (
	"context"
	"errors"
	"time"

	"github.com/agentstax/vulkan/pkg/alert"
	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/datastore"
	"github.com/agentstax/vulkan/pkg/produce"
)

// Record serializes classification and production on the owner's alert head.
// A nil finding resolves an active head; transition logs follow commit.
func (c *AlertController) Record(ctx context.Context, name string, owner *common.Owner, found *alert.Alert) (alert.RecordOutcome, error) {
	if owner == nil {
		return "", errors.New("owner must not be nil")
	}

	messageKey, err := alert.MessageKey(name, owner)
	if err != nil {
		return "", err
	}

	var head *common.StoredMessage[alert.Alert]
	var published *alert.Alert
	err = datastore.InTransaction(ctx, c.ds, func(ctx context.Context, tx datastore.Tx) error {
		var err error
		head, err = c.heads.LockHead[alert.Alert](ctx, tx, c.alerts.Topic.Id, messageKey)
		if err != nil {
			return err
		}

		published, err = classify(found, head, c.repeat, time.Now())
		if err != nil || published == nil {
			return err
		}

		compaction, err := produce.NewCompactionOptions(0)
		if err != nil {
			return err
		}
		_, err = c.alerts.ProduceInTx(ctx, tx, published, &produce.ProduceOptions{
			RoutingKey: published.RoutingKey(),
			MessageKey: messageKey,
			Compaction: compaction,
		})
		return err
	})
	if err != nil {
		return "", err
	}
	if published == nil {
		return alert.RecordOutcomeNothing, nil
	}

	if statusChanged(published, head) {
		c.logAlerts(ctx, published)
	}
	if published.Status == alert.AlertStatusResolved {
		return alert.RecordOutcomeResolved, nil
	}
	return alert.RecordOutcomeActive, nil
}

func (c *AlertController) logAlerts(ctx context.Context, published *alert.Alert) {
	if published.Status == alert.AlertStatusResolved {
		c.Logger.InfoContext(ctx, "alert resolved",
			"alert", published.Name, "alert_message", published.Message, "owner", published.Owner.Name)
		return
	}
	c.Logger.WarnContext(ctx, "alert active",
		"alert", published.Name, "alert_message", published.Message,
		"detail", published.Detail, "hint", published.Hint,
		"owner", published.Owner.Name, "severity", published.Severity)
}
