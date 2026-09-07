package controller

import (
	"testing"
	"time"
)

func TestGetMeasurementHistoryValidatesBeforeReading(t *testing.T) {
	controller := &MetricsController{}
	if _, err := controller.GetMeasurementHistory(t.Context(), "", time.Minute); err == nil {
		t.Fatal("empty key must fail before reading")
	}
	for _, window := range []time.Duration{0, -time.Minute} {
		if _, err := controller.GetMeasurementHistory(t.Context(), "orders", window); err == nil {
			t.Fatal("nonpositive window must fail before reading")
		}
	}
}
