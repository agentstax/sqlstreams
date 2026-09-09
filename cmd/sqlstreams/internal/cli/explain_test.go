package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentstax/sqlstreams/pkg/common/diagnostic"
)

var errTestStreamMissing = diagnostic.NewDiagnosticError("SQL9803", diagnostic.RecoveryPermanent,
	"test stream not found", "register it first",

	diagnostic.NewDiagnosticQuery("the stream rows registered under that name", `
SELECT id
FROM stream
WHERE name = '{stream}';`),
	diagnostic.NewDiagnosticQuery("the migration steps this database recorded", `
SELECT migration_version
FROM migration_log;`),
)

func TestRenderDiagnoseQueriesNamesEveryValueOnce(t *testing.T) {
	var builder strings.Builder
	renderDiagnoseQueries(&builder, errTestStreamMissing.Queries())

	want := "\ndiagnose: fill in stream with your own values\n" +
		"\n  -- the stream rows registered under that name\n" +
		"  SELECT id\n" +
		"  FROM stream\n" +
		"  WHERE name = '{stream}';\n" +
		"\n  -- the migration steps this database recorded\n" +
		"  SELECT migration_version\n" +
		"  FROM migration_log;\n"
	if builder.String() != want {
		t.Fatalf("got:\n%s\nwant:\n%s", builder.String(), want)
	}
}

func TestRenderDiagnoseQueriesDropsTheSubstitutionLineWithNothingToFillIn(t *testing.T) {
	var builder strings.Builder
	renderDiagnoseQueries(&builder, []diagnostic.DiagnosticQuery{
		*diagnostic.NewDiagnosticQuery("every registered stream", "SELECT name FROM stream_config;"),
	})

	want := "\ndiagnose:\n" +
		"\n  -- every registered stream\n" +
		"  SELECT name FROM stream_config;\n"
	if builder.String() != want {
		t.Fatalf("got:\n%s\nwant:\n%s", builder.String(), want)
	}
}

// Most declarations have nothing to look at, so the section is absent rather
// than empty.
func TestRenderDiagnoseQueriesWritesNothingWhenNoneAreDeclared(t *testing.T) {
	var builder strings.Builder
	renderDiagnoseQueries(&builder, errTestBroker.Queries())
	if builder.String() != "" {
		t.Fatalf("got:\n%s\nwant nothing", builder.String())
	}
}

func TestExplainDocumentCarriesTheDeclaredQueries(t *testing.T) {
	encoded, err := json.Marshal(toErrorExplainDocument(errTestStreamMissing, "register it first"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var document explainDocument
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatalf("output is not one json document: %v\n%s", encoded, err)
	}
	if len(document.Queries) != 2 {
		t.Fatalf("queries = %d, want 2", len(document.Queries))
	}
	first := document.Queries[0]
	if first.Label != "the stream rows registered under that name" {
		t.Errorf("label = %q", first.Label)
	}
	if !strings.HasPrefix(first.Sql, "SELECT id\n") {
		t.Errorf("sql = %q, want it to start at the SELECT", first.Sql)
	}
	if len(first.Placeholders) != 1 || first.Placeholders[0] != "stream" {
		t.Errorf("placeholders = %v, want [stream]", first.Placeholders)
	}
}

// A declaration with no queries leaves the key out rather than carrying an
// empty list, so a reader branches on presence.
func TestExplainDocumentOmitsQueriesWhenNoneAreDeclared(t *testing.T) {
	encoded, err := json.Marshal(toErrorExplainDocument(errTestBroker, "retry the produce call"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(encoded), "queries") {
		t.Fatalf("queries key present: %s", encoded)
	}
}

func TestMetricExplainDocumentCarriesCatalogMetadata(t *testing.T) {
	document := toMetricExplainDocument(metricTestDepth)

	if document.Scope != diagnostic.MetricScopeConsumerGroup {
		t.Fatalf("scope = %q", document.Scope)
	}
	if len(document.AttributeKeys) != 2 || document.AttributeKeys[0] != "stream" || document.AttributeKeys[1] != "group" {
		t.Fatalf("attribute keys = %v", document.AttributeKeys)
	}
	document.AttributeKeys[0] = "changed"
	if metricTestDepth.AttributeKeys[0] != "stream" {
		t.Fatalf("explain document mutated the declaration: %v", metricTestDepth.AttributeKeys)
	}
}
