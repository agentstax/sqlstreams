package checker

import (
	"time"

	"github.com/agentstax/vulkan/bench/reliability/checker/datastore"
	"github.com/agentstax/vulkan/bench/reliability/record"
	"github.com/agentstax/vulkan/bench/reliability/scenario"
)

// VerdictStatus is the run's one-word outcome. Unknown means the checks
// could not be run to completion; Reason says why.
type VerdictStatus string

const (
	VerdictStatusPass    VerdictStatus = "pass"
	VerdictStatusFail    VerdictStatus = "fail"
	VerdictStatusUnknown VerdictStatus = "unknown"
)

// Verdict is the record one checker run writes: what was judged, under what
// server setting and library version, and how each expectation came out.
type Verdict struct {
	Scenario          string                   `json:"scenario"`
	Status            VerdictStatus            `json:"status"`
	Reason            string                   `json:"reason"` // "" unless unknown
	StartedAt         time.Time                `json:"started_at"`
	Duration          time.Duration            `json:"duration_ns"`
	VulkanVersion     string                   `json:"vulkan_version"`
	SynchronousCommit string                   `json:"synchronous_commit"`
	Records           RecordSummary            `json:"records"`
	Produced          datastore.ProduceSummary `json:"produced"`
	Handled           datastore.HandlerSummary `json:"handled"`
	Checks            []CheckResult            `json:"checks"`
	Phases            []record.PhaseRecord     `json:"phases"`
}

// RecordSummary counts the rows loaded from the record files, per kind.
type RecordSummary struct {
	Produce int64 `json:"produce"`
	Handler int64 `json:"handler"`
	Phase   int64 `json:"phase"`
}

// ExitCode is the process exit code for the verdict: 0 pass, 1 fail,
// 2 unknown. The binary reserves 3 for a lab failure that left no verdict.
func (v *Verdict) ExitCode() int {
	switch v.Status {
	case VerdictStatusPass:
		return 0
	case VerdictStatusFail:
		return 1
	}
	return 2
}

// CheckStatus is one expectation's outcome: a want of 0 passes or fails on
// its count; a want of report is reported whatever the count.
type CheckStatus string

const (
	CheckStatusPass   CheckStatus = "pass"
	CheckStatusFail   CheckStatus = "fail"
	CheckStatusReport CheckStatus = "report"
)

// CheckResult is one [expect] line judged: the expectation as declared, the
// count the query returned, and up to exampleLimit examples of what it
// counted -- ExampleOf says whether they are keys or message ids.
type CheckResult struct {
	Check     scenario.Check `json:"check"`
	Want      scenario.Want  `json:"want"`
	Actual    int64          `json:"actual"`
	Status    CheckStatus    `json:"status"`
	ExampleOf string         `json:"example_of"`
	Examples  []string       `json:"examples"`
}

func newCheckResult(expectation scenario.Expectation, measured datastore.Measurement) CheckResult {
	status := CheckStatusReport
	if expectation.Want == scenario.WantZero {
		status = CheckStatusPass
		if measured.Count != 0 {
			status = CheckStatusFail
		}
	}
	return CheckResult{
		Check:     expectation.Check,
		Want:      expectation.Want,
		Actual:    measured.Count,
		Status:    status,
		ExampleOf: measured.ExampleOf,
		Examples:  measured.Examples,
	}
}

// ***************
// *** HELPERS ***
// ***************

// statusOf is fail when any check failed, else pass; unknown is decided
// before the checks run.
func statusOf(checks []CheckResult) VerdictStatus {
	for _, check := range checks {
		if check.Status == CheckStatusFail {
			return VerdictStatusFail
		}
	}
	return VerdictStatusPass
}
