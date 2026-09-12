package diagnostic

import (
	"slices"
	"strings"
	"sync"
)

const docsBaseURL = "https://sqlstreams.io/errors/"

// Declaration is one registered SQL-coded declaration. The registry stores
// any kind through this interface; retrieval by kind stays with each
// kind's own lister (Errors, Events, Metrics, Alerts).
type Declaration interface {
	GetCode() string
	GetKind() DiagnosticKind
}

// DiagnosticKind names a Declaration's kind.
type DiagnosticKind string

const (
	DiagnosticKindError  DiagnosticKind = "error"  // a declared error value (Err*)
	DiagnosticKindEvent  DiagnosticKind = "event"  // a declared log event (Event*)
	DiagnosticKindMetric DiagnosticKind = "metric" // a built-in metric
	DiagnosticKindAlert  DiagnosticKind = "alert"  // a built-in alert
)

// The SQL code registry: every declaration kind shares one serial space, so
// registering a code any kind already holds panics. Filled at package init
// by the New* constructors.
var (
	registryLock           sync.Mutex
	registeredDeclarations = map[string]Declaration{}
)

func register(declared Declaration) {
	code := declared.GetCode()
	if !isSQLCode(code) {
		panic(string(declared.GetKind()) + ` code must be "SQL" followed by four digits: ` + code)
	}

	registryLock.Lock()
	defer registryLock.Unlock()
	if existing, ok := registeredDeclarations[code]; ok {
		panic("code already registered as " + string(existing.GetKind()) + ": " + code)
	}
	registeredDeclarations[code] = declared
}

// listRegistered returns every registered declaration of the one concrete
// kind D, ordered by code.
func listRegistered[D Declaration]() []D {
	registryLock.Lock()
	defer registryLock.Unlock()

	listed := make([]D, 0, len(registeredDeclarations))
	for _, registered := range registeredDeclarations {
		if declared, ok := registered.(D); ok {
			listed = append(listed, declared)
		}
	}
	slices.SortFunc(listed, func(left D, right D) int {
		return strings.Compare(left.GetCode(), right.GetCode())
	})

	return listed
}

func isSQLCode(code string) bool {
	if len(code) != 7 || code[:3] != "SQL" {
		return false
	}
	for _, digit := range code[3:] {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}
