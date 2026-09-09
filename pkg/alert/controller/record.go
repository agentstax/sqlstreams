package controller

import (
	"context"
	"errors"
	"time"

	"github.com/agentstax/sqlstreams/pkg/alert"
	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/datastore"
	"github.com/agentstax/sqlstreams/pkg/produce"
)

// Record serializes classification and production on the owner's alert head.
// Only healthy results resolve an active head; transition logs follow commit.
func (c *AlertController) Record(ctx context.Context, name string, owner *common.Owner, result *alert.AlertEvaluationSnapshot) (alert.RecordOutcome, error) {
	if owner == nil {
		return "", errors.New("owner must not be nil")
	}
	if result == nil {
		return "", errors.New("result must not be nil")
	}
	if err := result.Validate(); err != nil {
		return "", err
	}
	if result.State == alert.AlertEvaluationStatePending || result.State == alert.AlertEvaluationStateInsufficientEvidence {
		return alert.RecordOutcomeNothing, nil
	}

	messageKey, err := alert.MessageKey(name, owner)
	if err != nil {
		return "", err
	}

	var head *common.StoredMessage[alert.Alert]
	var published *alert.Alert
	err = datastore.InTransaction(ctx, c.ds, func(ctx context.Context, tx datastore.Tx) error {
		var err error
		head, err = c.heads.LockHead[alert.Alert](ctx, tx, c.alerts.Stream.Id, messageKey)
		if err != nil {
			return err
		}

		published, err = classify(result.Finding, head, c.repeat, time.Now())
		if err != nil || published == nil {
			return err
		}

		_, err = c.alerts.ProduceInTx(ctx, tx, published, &produce.ProduceOptions{
			RoutingKey: published.RoutingKey(),
			MessageKey: messageKey,
			Compaction: &produce.CompactionOptions{Enable: true},
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
