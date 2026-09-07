package checker

import (
	"time"

	"github.com/agentstax/vulkan/bench/reliability/record"
)

// VerdictStatus is the run's one-word outcome. Unknown means the checks
// could not be run to completion; Reason says why.
type VerdictStatus string

const (
	VerdictPass    VerdictStatus = "pass"
	VerdictFail    VerdictStatus = "fail"
	VerdictUnknown VerdictStatus = "unknown"
)

// the verdict's exit codes; the binary reserves 3 for a lab failure
const (
	ExitPass    = 0
	ExitFail    = 1
	ExitUnknown = 2
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

func (v *Verdict) ExitCode() int {
	switch v.Status {
	case VerdictPass:
		return ExitPass
	case VerdictFail:
		return ExitFail
	}
	return ExitUnknown
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
