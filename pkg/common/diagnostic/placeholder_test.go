package diagnostic

import (
	"log/slog"
	"slices"
	"testing"
)

var errTestFixSubstitutes = NewDiagnosticError("SQL9903", RecoveryPermanent,
	"test schema version is older than this build requires",
	"migrate the {owner_kind} schema up from {version} to {build_version}")

// closed set: how a fix placeholder renders -- filled raw from an attached
// value, left literal when nothing attached its name.
func TestFixPlaceholdersFillFromAttachedValues(t *testing.T) {
	errTestFixQuoted := NewDiagnosticError("SQL9904", RecoveryPermanent,
		"test stream not found",
		`register "{stream}" with RegisterStream first`)
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "every placeholder attached", err: errTestFixSubstitutes.With("owner_kind", "stream", "version", 4, "build_version", 7), want: `test schema version is older than this build requires: owner_kind "stream", version 4, build_version 7 -- migrate the stream schema up from 4 to 7 [SQL9903]`},
		{name: "value goes in raw inside the fix's own quotes", err: errTestFixQuoted.With("stream", "orders"), want: `test stream not found: stream "orders" -- register "orders" with RegisterStream first [SQL9904]`},
		{name: "unattached placeholder stays literal", err: errTestFixSubstitutes.With("owner_kind", "stream"), want: `test schema version is older than this build requires: owner_kind "stream" -- migrate the stream schema up from {version} to {build_version} [SQL9903]`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.err.Error(); got != test.want {
				t.Fatalf("Error(%s) = %q, want %q", test.name, got, test.want)
			}
		})
	}
}

// behavior: the slog surface fills the fix the same way Error() does.
func TestLogValueFillsTheFix(t *testing.T) {
	raised := errTestFixSubstitutes.With("owner_kind", "system", "version", 1, "build_version", 2)

	var filled string
	for _, attribute := range raised.LogValue().Group() {
		if attribute.Key == "fix" {
			filled = attribute.Value.String()
		}
	}
	if want := "migrate the system schema up from 1 to 2"; filled != want {
		t.Fatalf("LogValue()[\"fix\"] = %q, want %q", filled, want)
	}
}

// closed set: the placeholder names a declaration reports, once each, in
// order; none for a static fix.
func TestFixPlaceholdersListsEachNameOnce(t *testing.T) {
	errTestFixRepeats := NewDiagnosticError("SQL9905", RecoveryPermanent,
		"test stream not found",
		"register {stream} again, or destroy {stream} first")
	tests := []struct {
		name     string
		declared *DiagnosticError
		want     []string
	}{
		{name: "three names", declared: errTestFixSubstitutes, want: []string{"owner_kind", "version", "build_version"}},
		{name: "repeated name", declared: errTestFixRepeats, want: []string{"stream"}},
		{name: "static fix", declared: errTestStreamMissing, want: []string{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.declared.FixPlaceholders(); !slices.Equal(got, test.want) {
				t.Fatalf("FixPlaceholders(%s) = %v, want %v", test.name, got, test.want)
			}
		})
	}
}

// regression: a JSONB containment literal in a diagnose query, or a Go
// composite literal in a fix, is a brace run and not a placeholder.
func TestFillLeavesANonAttributeBraceRunAlone(t *testing.T) {
	values := []slog.Attr{slog.String("stream", "orders")}
	text := `payload @> '{}'`

	if got := fillPlaceholders(text, values); got != text {
		t.Fatalf("fillPlaceholders(%q) = %q, want the text unchanged", text, got)
	}
}
