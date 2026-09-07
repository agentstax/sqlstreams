package checker

import (
	"fmt"
	"strings"

	"github.com/agentstax/vulkan/bench/reliability/scenario"
)

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

func newCheckResult(expectation scenario.Expectation, witness string, measured measurement) CheckResult {
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
		Witness:   witness,
		Witnesses: measured.Witnesses,
	}
}

// Beside is the report's columns after the declared line, tab-separated:
// the actual, then PASS or FAIL for a want of 0, then the witnesses of a
// non-zero count.
func (r CheckResult) Beside() string {
	columns := []string{fmt.Sprintf("actual %d", r.Actual)}
	switch r.Status {
	case CheckPassed:
		columns = append(columns, "PASS")
	case CheckFailed:
		columns = append(columns, "FAIL")
	}
	if len(r.Witnesses) > 0 {
		columns = append(columns, r.Witness+" "+strings.Join(r.Witnesses, ", "))
	}
	return strings.Join(columns, "\t")
}
