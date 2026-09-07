package scenario

import (
	"errors"
	"fmt"
)

// Check names one check the checker knows how to run. The set is
// closed: a new check is a checker change and a new const here.
type Check string

const (
	CheckLost        Check = "lost"
	CheckUndelivered Check = "undelivered"
	CheckUnbucketed  Check = "unbucketed"
	CheckDuplicates  Check = "duplicates"
	CheckReclaims    Check = "reclaims"
	CheckDead        Check = "dead"
)

func (c Check) Validate() error {
	switch c {
	case CheckLost, CheckUndelivered, CheckUnbucketed,
		CheckDuplicates, CheckReclaims, CheckDead:
		return nil
	}
	return fmt.Errorf("unrecognized check: %q", string(c))
}

// Expectation is one [expect] line: the check and the value the report prints
// beside the actual. Want is "0" for a count that must be zero and "report"
// for a count that is shown and never fails.
type Expectation struct {
	Check Check
	Want  string
}

func (e Expectation) Validate() error {
	if err := e.Check.Validate(); err != nil {
		return err
	}
	if e.Want == "" {
		return errors.New("Want is required")
	}
	return nil
}

// String is the [expect] line: "lost\t0", tab-separated for the section's
// column alignment.
func (e Expectation) String() string {
	return fmt.Sprintf("%s\t%s", e.Check, e.Want)
}
