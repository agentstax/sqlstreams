package diagnostic

import (
	"slices"
	"strings"
	"testing"
)

const deliverySql = `SELECT
	status,
	attempts
FROM exception_queue_{stream_id}
WHERE consumer_group_id = {group_id}
	AND message_id = {message_id};`

func TestNewQueryKeepsWhatItWasGiven(t *testing.T) {
	query := NewDiagnosticQuery("the delivery row", deliverySql)
	if query.Label != "the delivery row" {
		t.Errorf("label = %q, want %q", query.Label, "the delivery row")
	}
	if query.Sql != deliverySql {
		t.Error("sql was not kept verbatim")
	}
}

// A declaration writes its SQL on the line after the opening backtick, so
// the leading newline is the literal's shape, not the query's.
func TestNewQueryTrimsTheLiteralsOwnWhitespace(t *testing.T) {
	query := NewDiagnosticQuery("the delivery row", "\n"+deliverySql+"\n\t")
	if query.Sql != deliverySql {
		t.Errorf("sql = %q, want it trimmed", query.Sql)
	}
}

func TestNewQueryPanicsOnStructuralMistakes(t *testing.T) {
	cases := map[string]struct {
		label string
		sql   string
	}{
		"no label":        {"", "SELECT 1"},
		"no sql":          {"the delivery row", ""},
		"only whitespace": {"the delivery row", "\n\t"},
	}
	for name, one := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("NewDiagnosticQuery returned instead of panicking")
				}
			}()
			NewDiagnosticQuery(one.label, one.sql)
		})
	}
}

// A JSONB literal is written '{"key": 1}', so its braces must not read as a
// placeholder the reader is asked to fill in.
func TestQueryPlaceholdersSkipsJsonbLiterals(t *testing.T) {
	query := NewDiagnosticQuery("messages carrying the key", `SELECT id FROM message_log_{stream_id} WHERE payload @> '{"tenant": 1}'`)
	if got := query.Placeholders(); !slices.Equal(got, []string{"stream_id"}) {
		t.Errorf("placeholders = %v, want [stream_id]", got)
	}
}

func TestQueryPlaceholders(t *testing.T) {
	cases := map[string]struct {
		sql  string
		want []string
	}{
		"in first-appearance order": {deliverySql, []string{"stream_id", "group_id", "message_id"}},
		"each listed once":          {"{stream_id} {group_id} {stream_id}", []string{"stream_id", "group_id"}},
		"none to substitute":        {"SELECT migration_version FROM migration_log", []string{}},
	}
	for name, one := range cases {
		t.Run(name, func(t *testing.T) {
			got := NewDiagnosticQuery("a query", one.sql).Placeholders()
			if !slices.Equal(got, one.want) {
				t.Errorf("placeholders = %v, want %v", got, one.want)
			}
		})
	}
}

func TestConstructorsOwnDiagnosticQueries(t *testing.T) {
	query := NewDiagnosticQuery("the delivery row", deliverySql)
	declared := NewDiagnosticError("SS9001", RecoveryPermanent, "a condition with state to look at", "do the thing", query)
	event := NewDiagnosticEvent("SS9002", "a thing happened", "", query)
	query.Sql = "changed constructor input"
	raised := declared.With("stream", "orders").Wrap(nil)
	for _, queries := range [][]DiagnosticQuery{declared.Queries(), event.Queries(), raised.Queries()} {
		if len(queries) != 1 || queries[0].Sql != strings.TrimSpace(deliverySql) {
			t.Fatalf("constructor did not copy its queries: %v", queries)
		}
		queries[0].Sql = "changed returned snapshot"
	}
	for _, queries := range [][]DiagnosticQuery{declared.Queries(), event.Queries(), raised.Queries()} {
		if queries[0].Sql != strings.TrimSpace(deliverySql) {
			t.Fatal("query snapshot mutated shared declaration")
		}
	}
	registered := false
	for _, listed := range Errors() {
		if listed.GetCode() == "SS9001" {
			registered = len(listed.Queries()) == 1
		}
	}
	if !registered {
		t.Fatal("registry did not receive the completed declaration")
	}
}

func TestConstructorsRejectNilDiagnosticQueries(t *testing.T) {
	for name, declare := range map[string]func(){
		"error": func() { NewDiagnosticError("SS9003", RecoveryPermanent, "a condition", "", nil) },
		"event": func() { NewDiagnosticEvent("SS9004", "a condition", "", nil) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("nil query was accepted")
				}
			}()
			declare()
		})
	}
}
