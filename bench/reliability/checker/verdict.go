package checker

import (
	"time"

	"github.com/agentstax/vulkan/bench/reliability/record"
	"github.com/agentstax/vulkan/bench/reliability/scenario"
)

// VerdictStatus is the run's one-word outcome. Unknown means the checks
// could not be run to completion; Reason says why.
type VerdictStatus string

const (
	VerdictPass    VerdictStatus = "pass"
	VerdictFail    VerdictStatus = "fail"
	VerdictUnknown VerdictStatus = "unknown"
)

// Verdict is the record one checker run writes: what was judged, under what
// server setting and library version, and how each expectation came out.
type Verdict struct {
	Scenario          string         `json:"scenario"`
	Status            VerdictStatus  `json:"status"`
	Reason            string         `json:"reason"` // "" unless unknown
	StartedAt         time.Time      `json:"started_at"`
	Duration          time.Duration  `json:"duration_ns"`
	VulkanVersion     string         `json:"vulkan_version"`
	SynchronousCommit string         `json:"synchronous_commit"`
	Records           RecordSummary  `json:"records"`
	Produced          ProduceSummary `json:"produced"`
	Handled           HandlerSummary `json:"handled"`
	Checks            []CheckResult  `json:"checks"`
	Phases            []record.Phase `json:"phases"`
}

// ExitCode is the process exit code for the verdict: 0 pass, 1 fail,
// 2 unknown. The binary reserves 3 for a lab failure that left no verdict.
func (v *Verdict) ExitCode() int {
	switch v.Status {
	case VerdictPass:
		return 0
	case VerdictFail:
		return 1
	}
	return 2
}

// CheckStatus is one expectation's outcome: a want of 0 passes or fails on
// its count; a want of report is reported whatever the count.
type CheckStatus string

const (
	CheckPassed   CheckStatus = "pass"
	CheckFailed   CheckStatus = "fail"
	CheckReported CheckStatus = "report"
)

// CheckResult is one [expect] line judged: the expectation as declared, the
// count the query returned, and up to witnessLimit examples of what it
// counted -- Witness says whether they are keys or message ids.
type CheckResult struct {
	Check     scenario.Check `json:"check"`
	Want      scenario.Want  `json:"want"`
	Actual    int64          `json:"actual"`
	Status    CheckStatus    `json:"status"`
	Witness   string         `json:"witness"`
	Witnesses []string       `json:"witnesses"`
}

func newCheckResult(expectation scenario.Expectation, measured measurement) CheckResult {
	status := CheckReported
	if expectation.Want == scenario.WantZero {
		status = CheckPassed
		if measured.Count != 0 {
			status = CheckFailed
		}
	}
	return CheckResult{
		Check:     expectation.Check,
		Want:      expectation.Want,
		Actual:    measured.Count,
		Status:    status,
		Witness:   measured.Witness,
		Witnesses: measured.Witnesses,
	}
}

// ***************
// *** HELPERS ***
// ***************

// statusOf is fail when any check failed, else pass; unknown is decided
// before the checks run.
func statusOf(checks []CheckResult) VerdictStatus {
	for _, check := range checks {
		if check.Status == CheckFailed {
			return VerdictFail
		}
	}
	return VerdictPass
}
