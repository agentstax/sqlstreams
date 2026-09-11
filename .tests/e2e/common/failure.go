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

// Die fails the program with the message.
func Die(message string) {
	panic(Failure{Message: message})
}

// Must fails the program on a non-nil error.
func Must(err error) {
	if err != nil {
		Die(err.Error())
	}
}

// Assert fails the program with the formatted message when condition is
// false.
func Assert(condition bool, format string, args ...any) {
	if !condition {
		Die(fmt.Sprintf(format, args...))
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
