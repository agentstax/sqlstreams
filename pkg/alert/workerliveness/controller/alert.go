package controller

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/alert"
	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/metric"
)

// The crossing decision is the caller's -- an alert built from no unclaimed
// rows is a bug.
func newWorkerLivenessAlert(owner *common.Owner, unclaimed []*metric.UnclaimedWorkerMetadata, at time.Time) (*alert.Alert, error) {
	if len(unclaimed) == 0 {
		return nil, errors.New("unclaimed must not be empty")
	}

	rows := make([]map[string]any, 0, len(unclaimed))
	for _, snapshot := range unclaimed {
		rows = append(rows, map[string]any{
			"worker":           snapshot.Name,
			"owner":            snapshot.Owner.Name,
			"owner_kind":       string(snapshot.Owner.Kind()),
			"target_instances": snapshot.TargetInstances,
		})
	}

	message := fmt.Sprintf("stream %q has no live instance on %d of its worker rows", owner.Name, len(unclaimed))
	detail := fmt.Sprintf("Nothing is running: %s. A worker row with no live instance does no work: expired partitions are not dropped, exceptions are not retried, and the group's cursor stops advancing.", unclaimedByOwner(unclaimed))
	hint := "Run \"sqlstreams manager run\" in a process that stays up, or start a consumer on the stream -- either one claims these rows."
	data := map[string]any{
		"unclaimed_count": len(unclaimed),
		"workers":         rows,
	}
	return alert.NewAlert(alert.AlertWorkerLiveness.Name, owner, alert.AlertStatusActive, alert.AlertSeverity(alert.AlertWorkerLiveness.Severity), message, at, &alert.AlertOptions{
		Detail: detail,
		Hint:   hint,
		Data:   data,
	})
}

// unclaimedByOwner renders the rows as "<owner> (<worker>, <worker>)" so one
// dark consumer group reads as one entry, not four.
func unclaimedByOwner(unclaimed []*metric.UnclaimedWorkerMetadata) string {
	owners := make([]string, 0, len(unclaimed))
	workers := map[string][]string{}
	for _, snapshot := range unclaimed {
		if _, seen := workers[snapshot.Owner.Name]; !seen {
			owners = append(owners, snapshot.Owner.Name)
		}
		workers[snapshot.Owner.Name] = append(workers[snapshot.Owner.Name], snapshot.Name)
	}

	entries := make([]string, 0, len(owners))
	for _, name := range owners {
		entries = append(entries, fmt.Sprintf("%s (%s)", name, strings.Join(workers[name], ", ")))
	}
	return strings.Join(entries, ", ")
}
