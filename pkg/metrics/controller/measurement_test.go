package controller

import (
	"strings"
	"testing"
	"time"
)

func TestListMeasurementMessagesByCreatedAtValidatesBeforeReading(t *testing.T) {
	start := time.Date(2026, time.September, 7, 10, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Minute)
	tests := []struct {
		name  string
		key   string
		start time.Time
		end   time.Time
		want  string
	}{
		{name: "missing key", start: start, end: end, want: "messageKey must not be empty"},
		{name: "missing start", key: "orders", end: end, want: "start must not be zero"},
		{name: "missing end", key: "orders", start: start, want: "end must not be zero"},
		{name: "reversed interval", key: "orders", start: end, end: start, want: "end must be >= start"},
	}
	controller := &MetricsController{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := controller.ListMeasurementMessagesByCreatedAt(t.Context(), test.key, test.start, test.end)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ListMeasurementMessagesByCreatedAt() = %v, want %q", err, test.want)
			}
		})
	}
}
