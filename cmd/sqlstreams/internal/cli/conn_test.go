package cli

import (
	"log/slog"
	"strings"
	"testing"
)

// closed set: every usage error the connection settings can raise, each
// rejected before anything dials.
func TestConnectionRejectsInvalidSettingsBeforeDial(t *testing.T) {
	t.Setenv(databaseURLEnv, "")
	t.Setenv(schemaEnv, "")
	for _, sample := range []struct {
		name   string
		url    string
		schema string
		want   string
	}{
		{"missing URL", "", "", "no database URL"},
		{"invalid schema", "postgres://localhost/unused", "invalid-schema", "Schema must be"},
		{"wrong scheme", "https://localhost/unused", "", "must start with postgres"},
		{"search path", "postgres://localhost/unused?search_path=other", "", "sets search_path"},
	} {
		t.Run(sample.name, func(t *testing.T) {
			poolConfig, datastoreConfig, err := resolveConnection(sample.url, sample.schema, slog.LevelError)
			if poolConfig != nil || datastoreConfig != nil || err == nil || !strings.Contains(err.Error(), sample.want) {
				t.Fatalf("resolveConnection(%q, %q) = %v, %v, %v, want a usage error containing %q", sample.url, sample.schema, poolConfig, datastoreConfig, err, sample.want)
			}
		})
	}
}

// behavior: a flag wins over its environment variable, and an empty flag
// falls back to it -- for the URL and the schema alike.
func TestConnectionResolvesFlagsBeforeTheEnvironment(t *testing.T) {
	// setup
	t.Setenv(databaseURLEnv, "postgres://from-environment:5433/unused")
	t.Setenv(schemaEnv, "from_environment")

	// test
	fromFlags, flagConfig, err := resolveConnection("postgres://from-flag:5432/unused", "from_flag", slog.LevelError)
	if err != nil {
		t.Fatal(err)
	}
	fromEnvironment, environmentConfig, err := resolveConnection("", "", slog.LevelError)

	// verify
	if err != nil {
		t.Fatal(err)
	}
	if fromFlags.ConnConfig.Host != "from-flag" || flagConfig.Schema != "from_flag" {
		t.Errorf("resolveConnection(flags) = host %q, schema %q, want from-flag and from_flag", fromFlags.ConnConfig.Host, flagConfig.Schema)
	}
	if fromEnvironment.ConnConfig.Host != "from-environment" || environmentConfig.Schema != "from_environment" {
		t.Errorf("resolveConnection(empty flags) = host %q, schema %q, want from-environment and from_environment", fromEnvironment.ConnConfig.Host, environmentConfig.Schema)
	}
}
