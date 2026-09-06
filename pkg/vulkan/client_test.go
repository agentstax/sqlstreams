package vulkan

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestClientCapturesConfiguration(t *testing.T) {
	url := os.Getenv("VULKAN_CLI_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set VULKAN_CLI_TEST_DATABASE_URL for the client integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	var output bytes.Buffer
	policy := &RetryPolicy{MaxRetries: 2}
	cfg := &ClientConfig{
		Schema:         "captured_config",
		DisableManager: true,
		Logger:         slog.New(slog.NewJSONHandler(&output, nil)),
		Retry:          policy,
	}
	client, err := NewClient(ctx, pool, cfg)
	if err != nil {
		t.Fatal(err)
	}
	policy.MaxRetries = 9
	cfg.Schema = "changed_config"
	cfg.DisableManager = false
	cfg.Logger = slog.Default()
	cfg.Retry = nil

	if client.ds.Schema != "captured_config" || !client.disableManager {
		t.Fatal("editing input config changed the client's captured settings")
	}
	if client.ds.Retry.MaxRetries != 2 || client.ds.Retry.BaseDelay <= 0 {
		t.Fatal("client did not retain an independently defaulted retry policy")
	}
	client.ds.Logger.WarnContext(ctx, "captured logger")
	if !strings.Contains(output.String(), `"schema":"captured_config"`) || !strings.Contains(output.String(), "captured logger") {
		t.Fatalf("original logger or schema attribution lost: %s", output.String())
	}

	other, err := NewClient(ctx, pool, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if other.disableManager || other.ds.Schema != "changed_config" {
		t.Fatal("new client did not use the edited config")
	}
	if client.ds.Pool != other.ds.Pool {
		t.Fatal("clients did not retain the caller's shared pool")
	}
}
