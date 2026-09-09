package diagnostic

import (
	"log/slog"
	"slices"
	"testing"
)

var errTestFixSubstitutes = NewDiagnosticError("SQL9903", RecoveryPermanent,
	"test schema version is older than this build requires",
	"migrate the {owner_kind} schema up from {version} to {build_version}")

func TestErrorFillsFixFromAttachedValues(t *testing.T) {
	raised := errTestFixSubstitutes.With("owner_kind", "stream", "version", 4, "build_version", 7)

	want := `test schema version is older than this build requires: owner_kind "stream", version 4, build_version 7 -- migrate the stream schema up from 4 to 7 [SQL9903]`
	if raised.Error() != want {
		t.Fatalf("got %q, want %q", raised.Error(), want)
	}
}

func TestFixSubstitutionKeepsTheValueRaw(t *testing.T) {
	declared := NewDiagnosticError("SQL9904", RecoveryPermanent,
		"test stream not found",
		`register "{stream}" with RegisterStream first`)
	raised := declared.With("stream", "orders")

	want := `test stream not found: stream "orders" -- register "orders" with RegisterStream first [SQL9904]`
	if raised.Error() != want {
		t.Fatalf("got %q, want %q", raised.Error(), want)
	}
}

func TestUnattachedPlaceholderStaysLiteral(t *testing.T) {
	raised := errTestFixSubstitutes.With("owner_kind", "stream")

	want := `test schema version is older than this build requires: owner_kind "stream" -- migrate the stream schema up from {version} to {build_version} [SQL9903]`
	if raised.Error() != want {
		t.Fatalf("got %q, want %q", raised.Error(), want)
	}
}

func TestLogValueFillsTheFix(t *testing.T) {
	raised := errTestFixSubstitutes.With("owner_kind", "system", "version", 1, "build_version", 2)

	var filled string
	for _, attribute := range raised.LogValue().Group() {
		if attribute.Key == "fix" {
			filled = attribute.Value.String()
		}
	}

	want := "migrate the system schema up from 1 to 2"
	if filled != want {
		t.Fatalf("got %q, want %q", filled, want)
	}
}

func TestFixPlaceholdersListsEachNameOnce(t *testing.T) {
	declared := NewDiagnosticError("SQL9905", RecoveryPermanent,
		"test stream not found",
		"register {stream} again, or destroy {stream} first")

	want := []string{"stream"}
	if got := declared.FixPlaceholders(); !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestFixPlaceholdersIsEmptyForAStaticFix(t *testing.T) {
	if got := errTestStreamMissing.FixPlaceholders(); len(got) != 0 {
		t.Fatalf("got %v, want none", got)
	}
}

// a jsonb containment literal in a query, a Go composite literal in a fix
func TestFillLeavesANonAttributeBraceRunAlone(t *testing.T) {
	values := []slog.Attr{slog.String("stream", "orders")}

	if got := fillPlaceholders(`payload @> '{}'`, values); got != `payload @> '{}'` {
		t.Fatalf("got %q, want the text unchanged", got)
	}
}
