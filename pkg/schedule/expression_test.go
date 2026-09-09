package schedule

import (
	"math"
	"testing"
	"time"
)

// behavior: an expression without a TZ prefix is read in UTC, whatever zone
// the caller's time carries.
func TestParseExpressionReadsUnzonedExpressionsInUTC(t *testing.T) {
	schedule, err := ParseExpression("0 9 * * *")
	if err != nil {
		t.Fatal(err)
	}

	// 08:00 UTC expressed in a +05:30 zone: the next 09:00 is 09:00 UTC, not
	// 09:00 in the caller's zone
	after := time.Date(2026, 3, 2, 13, 30, 0, 0, time.FixedZone("IST", 5*3600+1800))
	want := time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC)
	if got := schedule.Next(after); !got.Equal(want) {
		t.Fatalf("Next(%v) = %v, want %v", after, got, want)
	}
}

// closed set: the shortest gap between scheduled times, including the
// fall-back day of a zoned daily schedule and a schedule too rare to measure.
func TestMinRateBySchedule(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want time.Duration
	}{
		{name: "every minute", expr: "* * * * *", want: time.Minute},
		{name: "every five minutes", expr: "*/5 * * * *", want: 5 * time.Minute},
		{name: "hourly descriptor", expr: "@hourly", want: time.Hour},
		{name: "daily in UTC", expr: "0 0 * * *", want: 24 * time.Hour},
		{name: "every descriptor", expr: "@every 90s", want: 90 * time.Second},
		{name: "daily in New York on the fall-back day", expr: "TZ=America/New_York 0 0 * * *", want: 23 * time.Hour},
		{name: "leap day recurs once in four years", expr: "0 0 29 2 *", want: time.Duration(math.MaxInt64)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			schedule, err := ParseExpression(test.expr)
			if err != nil {
				t.Fatalf("ParseExpression(%q) = %v, want nil", test.expr, err)
			}
			if got := schedule.MinRate(); got != test.want {
				t.Fatalf("ParseExpression(%q).MinRate() = %v, want %v", test.expr, got, test.want)
			}
		})
	}
}

// closed set: the expressions ParseExpression refuses -- malformed, faster
// than the producer's one-minute resolution, or never coming due.
func TestParseExpressionRejectsUnschedulableExpressions(t *testing.T) {
	tests := []struct {
		name string
		expr string
	}{
		{name: "malformed", expr: "not a cron expr"},
		{name: "faster than one minute", expr: "@every 30s"},
		{name: "never comes due", expr: "0 0 30 2 *"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ParseExpression(test.expr); err == nil {
				t.Fatalf("ParseExpression(%q) = nil, want error", test.expr)
			}
		})
	}
}
