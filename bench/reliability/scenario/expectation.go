package scenario

import "fmt"

// Check names one check the checker knows how to run. The set is
// closed: a new check is a checker change and a new const here.
//
//	lost         committed produces whose message_log row is missing
//	unexpected   message_log rows the ledger never committed or lost track of
//	recovered    unknown produces (reply lost) whose row is there after all
//	undelivered  messages the handler never succeeded on and the library never dead-lettered
//	duplicates   messages the handler succeeded on more than once
//	unbucketed   by the library's tables, messages in no bucket or in two (success and dead)
//	reclaims     deliveries logged expired -- a lease a consumer stopped renewing
//	dead         exception_queue rows dead-lettered
type Check string

const (
	CheckLost        Check = "lost"
	CheckUnexpected  Check = "unexpected"
	CheckRecovered   Check = "recovered"
	CheckUndelivered Check = "undelivered"
	CheckDuplicates  Check = "duplicates"
	CheckUnbucketed  Check = "unbucketed"
	CheckReclaims    Check = "reclaims"
	CheckDead        Check = "dead"
)

func (c Check) Validate() error {
	switch c {
	case CheckLost, CheckUnexpected, CheckRecovered, CheckUndelivered,
		CheckDuplicates, CheckUnbucketed, CheckReclaims, CheckDead:
		return nil
	}
	return fmt.Errorf("unrecognized check: %q", string(c))
}

// WantZero and WantReport are the two values an Expectation can want: a
// count that must be zero, or a count that is shown and never fails.
const (
	WantZero   = "0"
	WantReport = "report"
)

// Invariants are the expectations every scenario must declare, with these
// wants: the safety checks hold for any run, so a scenario cannot drop one
// and pass while checking less. The rest of [expect] is the scenario's own.
var Invariants = []Expectation{
	{Check: CheckLost, Want: WantZero},
	{Check: CheckUnexpected, Want: WantZero},
	{Check: CheckRecovered, Want: WantReport},
	{Check: CheckUndelivered, Want: WantZero},
	{Check: CheckDuplicates, Want: WantReport},
	{Check: CheckUnbucketed, Want: WantZero},
}

// Expectation is one [expect] line: the check and the value the report prints
// beside the actual.
type Expectation struct {
	Check Check
	Want  string
}

func (e Expectation) Validate() error {
	if err := e.Check.Validate(); err != nil {
		return err
	}
	if e.Want != WantZero && e.Want != WantReport {
		return fmt.Errorf("Want must be %q or %q, got %q", WantZero, WantReport, e.Want)
	}
	return nil
}

// String is the [expect] line: "lost\t0", tab-separated for the section's
// column alignment.
func (e Expectation) String() string {
	return fmt.Sprintf("%s\t%s", e.Check, e.Want)
}
