package diagnostic

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

var (
	errTestStreamMissing = NewDiagnosticError("SQL9901", RecoveryPermanent,
		"test stream not found",
		"register it with RegisterStream first")
	errTestConnection = NewDiagnosticError("SQL9902", RecoveryTransient,
		"could not reach the test broker", "")
)

// closed set: the Error() line for each combination of parts a raise can
// carry -- values, fix, wrapped cause.
func TestErrorRendersEachPartCombination(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "all parts", err: errTestStreamMissing.With("stream", "orders", "version", 3), want: `test stream not found: stream "orders", version 3 -- register it with RegisterStream first [SQL9901]`},
		{name: "no values", err: errTestStreamMissing, want: "test stream not found -- register it with RegisterStream first [SQL9901]"},
		{name: "no fix", err: errTestConnection.With("host", "db.local", "timeout", 5*time.Second), want: `could not reach the test broker: host "db.local", timeout 5s [SQL9902]`},
		{name: "wrapped cause", err: errTestConnection.With("host", "db.local").Wrap(errors.New("connection refused")), want: `could not reach the test broker: host "db.local" [SQL9902]: connection refused`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.err.Error(); got != test.want {
				t.Fatalf("Error(%s) = %q, want %q", test.name, got, test.want)
			}
		})
	}
}

// closed set: the fields LogValue renders for slog, the same parts as fields.
func TestLogValueRendersPartsAsFields(t *testing.T) {
	raised := errTestStreamMissing.With("stream", "orders").Wrap(errors.New("row deleted"))

	fields := map[string]string{}
	for _, attribute := range raised.LogValue().Group() {
		fields[attribute.Key] = attribute.Value.String()
	}
	want := map[string]string{
		"code":     "SQL9901",
		"problem":  "test stream not found",
		"recovery": "permanent",
		"docs":     "https://vulkan-5ss.pages.dev/errors/SQL9901",
		"fix":      "register it with RegisterStream first",
		"stream":   "orders",
		"cause":    "row deleted",
	}
	for key, value := range want {
		if fields[key] != value {
			t.Errorf("LogValue()[%q] = %q, want %q", key, fields[key], value)
		}
	}
}

// behavior: errors.Is matches a raise to its declaration by code, through
// fmt.Errorf wrapping, and reaches a wrapped cause; distinct codes never match.
func TestErrorsIsMatchesDeclarationByCode(t *testing.T) {
	cause := errors.New("connection refused")
	raised := errTestStreamMissing.With("stream", "orders")
	tests := []struct {
		name   string
		err    error
		target error
		want   bool
	}{
		{name: "raise to declaration", err: raised, target: errTestStreamMissing, want: true},
		{name: "fmt.Errorf wrapped raise to declaration", err: fmt.Errorf("list streams: %w", raised), target: errTestStreamMissing, want: true},
		{name: "wrapped cause", err: errTestConnection.Wrap(cause), target: cause, want: true},
		{name: "distinct codes", err: raised, target: errTestConnection, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := errors.Is(test.err, test.target); got != test.want {
				t.Fatalf("errors.Is(%s) = %v, want %v", test.name, got, test.want)
			}
		})
	}
}

// invariant: a declaration is a shared package variable, so With and Wrap
// return copies and its own rendering never changes.
func TestWithAndWrapNeverMutateTheDeclaration(t *testing.T) {
	before := errTestConnection.Error()
	cause := errors.New("connection refused")

	first := errTestConnection.With("host", "db.local")
	second := errTestConnection.With("host", "db.remote")
	errTestConnection.Wrap(cause)

	if got := errTestConnection.Error(); got != before {
		t.Errorf("declaration Error() after With and Wrap = %q, want %q", got, before)
	}
	if first.Error() == second.Error() {
		t.Errorf("two raises render alike: %q", first.Error())
	}
	if errors.Is(errTestConnection, cause) {
		t.Error("errors.Is(declaration, cause) = true after Wrap, want false")
	}
}
