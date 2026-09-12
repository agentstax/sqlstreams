package cli

import (
	"io"
	"testing"
)

func TestLatestReadsRejectHistoryLimit(t *testing.T) {
	// setup
	_, globals := newRootCmd()
	metric := newMetricReadCmd(globals, "latest")
	alert := newAlertReadCmd(globals, "latest")

	// test
	metricError := metric.ParseFlags([]string{"--limit", "5"})
	alertError := alert.ParseFlags([]string{"--limit", "5"})

	// verify
	if metricError == nil {
		t.Error("metric latest ParseFlags(--limit 5) = nil, want rejection")
	}
	if alertError == nil {
		t.Error("alert latest ParseFlags(--limit 5) = nil, want rejection")
	}
}

func TestSchedulerGetRejectsMessageListingFlags(t *testing.T) {
	// setup
	_, globals := newRootCmd()
	get := newScheduleGetCmd(globals)

	// test
	messagesError := get.ParseFlags([]string{"--messages"})
	limitError := get.ParseFlags([]string{"--limit", "5"})

	// verify
	if messagesError == nil {
		t.Error("scheduler get ParseFlags(--messages) = nil, want rejection")
	}
	if limitError == nil {
		t.Error("scheduler get ParseFlags(--limit 5) = nil, want rejection")
	}
}

func TestMetricSeriesLimitAcceptsExplicitCount(t *testing.T) {
	// setup
	_, globals := newRootCmd()
	history := newMetricReadCmd(globals, "history")

	// test
	err := history.ParseFlags([]string{"--series-limit", "3", "--limit", "5"})
	seriesLimit, seriesError := history.Flags().GetInt("series-limit")
	limit, limitError := history.Flags().GetInt("limit")

	// verify
	if err != nil {
		t.Fatalf("metric history ParseFlags(--series-limit 3 --limit 5) = %v, want nil", err)
	}
	if seriesError != nil || seriesLimit != 3 {
		t.Errorf("GetInt(series-limit) = %d, %v, want 3, nil", seriesLimit, seriesError)
	}
	if limitError != nil || limit != 5 {
		t.Errorf("GetInt(limit) = %d, %v, want 5, nil", limit, limitError)
	}
}

func TestUnrecognizedNestedCommandReturnsUsageError(t *testing.T) {
	// setup
	root, _ := newRootCmd()
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"metric", "get", "sample"})

	// test
	err := root.Execute()

	// verify
	if err == nil || exitCode(err) != 2 {
		t.Errorf("Execute(metric get sample) = %v, want usage error with exit 2", err)
	}
}
