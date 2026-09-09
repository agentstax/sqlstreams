package sqlstreams_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/agentstax/sqlstreams/pkg/sqlstreams"
	"github.com/agentstax/sqlstreams/pkg/sqlstreamstest"
)

func TestClientCapturesConfiguration(t *testing.T) {
	pool := sqlstreamstest.NewDatastore(t, nil).Pool
	ctx := t.Context()

	var output bytes.Buffer
	policy := &sqlstreams.RetryPolicy{MaxRetries: 2}
	cfg := &sqlstreams.ClientConfig{
		Schema:         "captured_config",
		DisableManager: true,
		Logger:         slog.New(slog.NewJSONHandler(&output, nil)),
		Retry:          policy,
	}
	client, err := sqlstreams.NewClient(ctx, pool, cfg)
	if err != nil {
		t.Fatal(err)
	}
	policy.MaxRetries = 9
	cfg.Schema = "changed_config"
	cfg.DisableManager = false
	cfg.Logger = slog.Default()
	cfg.Retry = nil

	if client.Datastore().Schema != "captured_config" || !client.ManagerDisabled() {
		t.Fatal("editing input config changed the client's captured settings")
	}
	if client.Datastore().Retry.MaxRetries != 2 || client.Datastore().Retry.BaseDelay <= 0 {
		t.Fatal("client did not retain an independently defaulted retry policy")
	}
	client.Datastore().Logger.WarnContext(ctx, "captured logger")
	if !strings.Contains(output.String(), `"schema":"captured_config"`) || !strings.Contains(output.String(), "captured logger") {
		t.Fatalf("original logger or schema attribution lost: %s", output.String())
	}

	other, err := sqlstreams.NewClient(ctx, pool, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if other.ManagerDisabled() || other.Datastore().Schema != "changed_config" {
		t.Fatal("new client did not use the edited config")
	}
	if client.Datastore().Pool != other.Datastore().Pool {
		t.Fatal("clients did not retain the caller's shared pool")
	}
}
