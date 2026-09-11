package checker

import (
	"strings"
	"testing"
)

func TestCPUReportDistinguishesMissingSamplesFromZeroUsage(t *testing.T) {
	zero := 0.0
	for _, test := range []struct {
		name    string
		percent *float64
		want    string
	}{
		{"missing", nil, "cpu postgres unavailable"},
		{"idle", &zero, "cpu postgres 0%"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// setup
			phase := PhaseSummary{PostgresCpu: test.percent}

			// test
			report := phase.ReportColumns()

			// verify
			if !strings.Contains(report, test.want) {
				t.Errorf("ReportColumns(%+v) = %q, want %q", phase, report, test.want)
			}
		})
	}
}
