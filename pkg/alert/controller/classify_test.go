package controller

import (
	"testing"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/alert"
	"github.com/allegedlyreliable/sqlstreams/pkg/common"
)

// closed set: what a run publishes against the head it last published --
// a found alert publishes over no head, a resolved head, a changed
// severity, or an elapsed repeat interval, and is held back otherwise;
// nothing found resolves an active head and leaves any other head alone.
func TestClassifyPublishesOnlyOnATransitionOrAnElapsedRepeat(t *testing.T) {
	const repeat = time.Hour
	tests := []struct {
		name         string
		found        bool
		head         alert.AlertStatus // "" for no head
		headSeverity alert.AlertSeverity
		headAge      time.Duration
		want         alert.AlertStatus // "" for nothing published
	}{
		{name: "found, no head", found: true, want: alert.AlertStatusActive},
		{name: "found, resolved head", found: true, head: alert.AlertStatusResolved, headSeverity: alert.AlertSeverityWarn, want: alert.AlertStatusActive},
		{name: "found, active head inside the repeat", found: true, head: alert.AlertStatusActive, headSeverity: alert.AlertSeverityWarn, headAge: repeat / 2, want: ""},
		{name: "found, active head past the repeat", found: true, head: alert.AlertStatusActive, headSeverity: alert.AlertSeverityWarn, headAge: repeat, want: alert.AlertStatusActive},
		{name: "found, active head at another severity", found: true, head: alert.AlertStatusActive, headSeverity: "critical", headAge: repeat / 2, want: alert.AlertStatusActive},
		{name: "nothing found, no head", found: false, want: ""},
		{name: "nothing found, resolved head", found: false, head: alert.AlertStatusResolved, headSeverity: alert.AlertSeverityWarn, want: ""},
		{name: "nothing found, active head", found: false, head: alert.AlertStatusActive, headSeverity: alert.AlertSeverityWarn, want: alert.AlertStatusResolved},
	}
	owner, err := common.NewStreamOwner(1, 1, "orders")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// setup
			var found *alert.Alert
			if test.found {
				found, err = alert.NewAlert("partition_count", owner, alert.AlertStatusActive, alert.AlertSeverityWarn, "partition count is high", now, nil)
				if err != nil {
					t.Fatal(err)
				}
			}
			var head *common.StoredMessage[alert.Alert]
			if test.head != "" {
				head = &common.StoredMessage[alert.Alert]{
					Message:   &alert.Alert{Name: "partition_count", Owner: owner, Status: test.head, Severity: test.headSeverity, Message: "partition count is high"},
					CreatedAt: now.Add(-test.headAge),
				}
			}

			// test
			published, err := classify(found, head, repeat, now)

			// verify
			if err != nil {
				t.Fatal(err)
			}
			var got alert.AlertStatus
			if published != nil {
				got = published.Status
			}
			if got != test.want {
				t.Fatalf("classify(found %t, head %q, severity %q, age %v) = %q, want %q", test.found, test.head, test.headSeverity, test.headAge, got, test.want)
			}
		})
	}
}
