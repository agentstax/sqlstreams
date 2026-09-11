package common

import (
	"fmt"
)

// Failure is what Die panics with; Recover turns it into run's error so
// main's deferred cleanup runs on a failed assertion.
type Failure struct {
	Message string
}

func (f Failure) Error() string {
	return f.Message
}

// Die fails the program with the formatted message.
func Die(format string, args ...any) {
	panic(Failure{Message: fmt.Sprintf(format, args...)})
}

// Must fails the program on a non-nil error.
func Must(err error) {
	if err != nil {
		Die("%s", err.Error())
	}
}

// Assert fails the program with the formatted message when condition is
// false.
func Assert(condition bool, format string, args ...any) {
	if !condition {
		Die(format, args...)
	}
}

// Recover is deferred first in run: a Failure becomes run's error, anything
// else keeps panicking.
//
//	func run() (err error) {
//		defer common.Recover(&err)
func Recover(err *error) {
	switch recovered := recover().(type) {
	case nil:
	case Failure:
		*err = recovered
	default:
		panic(recovered)
	}
}
