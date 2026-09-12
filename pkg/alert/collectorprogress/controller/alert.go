package controller

import (
	"fmt"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/alert"
	"github.com/allegedlyreliable/sqlstreams/pkg/common"
)

func newCollectorProgressAlert(owner *common.Owner, completedAt time.Time, maximumAge time.Duration, at time.Time) (*alert.Alert, error) {
	message := "metrics collector has no retained completed pass"
	detail := "No retained collector completion is available while system-manager leases remain continuous."
	if !completedAt.IsZero() {
		message = "metrics collector completion is overdue"
		detail = fmt.Sprintf("The last completed pass was at %s; the maximum completion age is %v.", completedAt.Format(time.RFC3339), maximumAge)
	}
	return alert.NewAlert(alert.AlertMetricsCollectorProgress.Name, owner, alert.AlertStatusActive, alert.AlertSeverityWarn, message, at, &alert.AlertOptions{
		Detail: detail,
		Hint:   "Inspect the metrics_collector worker and its failure logs; restore successful collection.",
		Data:   map[string]any{"completed_at": completedAt, "maximum_age": maximumAge},
	})
}
