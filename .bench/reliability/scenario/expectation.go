package scenario

import "fmt"

// Check names one check the checker knows how to run. The set is
// closed: a new check is a checker change and a new const here.
//
//	lost         committed produces whose message_log row is missing
//	unexpected   message_log rows the records never committed or lost track of
//	recovered    unknown produces (reply lost) whose row is there after all
//	undelivered  messages the handler never succeeded on and the library never dead-lettered
//	duplicates   messages the handler succeeded on more than once
//	unbucketed   messages neither completed by the durable cursor nor dead-lettered
//	reclaims     deliveries logged expired -- a lease a consumer stopped renewing
//	dead         exception_queue rows dead-lettered
//	schedule_kept       seconds a produce started more than 100ms behind its scheduled instant -- the generator, not the library, was the limiter
//	backlog_bounded     producer phases in which the group's backlog trended up faster than 5% of the declared rate -- the consumers did not keep up
//	generator_headroom  container samples in which the producer or consumer sat above 80% of its CPU cap -- the generator, not the server, was the limiter
type Check string

const (
	CheckErrors            Check = "errors"
	CheckLost              Check = "lost"
	CheckUnexpected        Check = "unexpected"
	CheckRecovered         Check = "recovered"
	CheckUndelivered       Check = "undelivered"
	CheckDuplicates        Check = "duplicates"
	CheckUnbucketed        Check = "unbucketed"
	CheckReclaims          Check = "reclaims"
	CheckDead              Check = "dead"
	CheckScheduleKept      Check = "schedule_kept"
	CheckBacklogBounded    Check = "backlog_bounded"
	CheckGeneratorHeadroom Check = "generator_headroom"
)

func (c Check) Validate() error {
	switch c {
	case CheckErrors, CheckLost, CheckUnexpected, CheckRecovered, CheckUndelivered,
		CheckDuplicates, CheckUnbucketed, CheckReclaims, CheckDead,
		CheckScheduleKept, CheckBacklogBounded, CheckGeneratorHeadroom:
		return nil
	}
	return fmt.Errorf("unrecognized check: %q", string(c))
}

// Want is what an Expectation asks of its count: zero, or shown and never
// failed.
type Want string

const (
	WantZero   Want = "0"
	WantReport Want = "report"
)

func (w Want) Validate() error {
	switch w {
	case WantZero, WantReport:
		return nil
	}
	return fmt.Errorf("unrecognized want: %q", string(w))
}

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

// Expectation is one [expect] line: the check and what its count must be.
type Expectation struct {
	Check Check
	Want  Want
}

func (e Expectation) Validate() error {
	if err := e.Check.Validate(); err != nil {
		return err
	}
	return e.Want.Validate()
}

// String is the [expect] line: "lost\t0", tab-separated for the section's
// column alignment.
func (e Expectation) String() string {
	return fmt.Sprintf("%s\t%s", e.Check, e.Want)
}
