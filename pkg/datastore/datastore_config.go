package datastore

import (
	"fmt"
	"os"
	"regexp"

	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/common/logging"
)

const DefaultSchema = "vulkan"

// schemaName is the identifier Postgres accepts unquoted and lowercased
var schemaNamePattern = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

type PostgresDatastoreConfig struct {
	// Schema - the Postgres namespace holding every table.
	// Default: "vulkan".
	Schema string

	// Logger - your own *slog.Logger or anything satisfying logging.Logger.
	// Held once here and read by every layer built over this datastore.
	// Default: text lines to stderr, warn level and up.
	Logger logging.Logger

	// Retry - transient-error retry policy for every Postgres call made
	// through this datastore, never a message's redelivery. Held once,
	// like Logger.
	// Default: common.NewDefaultRetryPolicy().
	Retry *common.RetryPolicy
}

// WithDefaults fills Schema (DefaultSchema), Logger, and Retry.
func (c *PostgresDatastoreConfig) WithDefaults() *PostgresDatastoreConfig {
	if c.Schema == "" {
		c.Schema = DefaultSchema
	}
	if c.Logger == nil {
		c.Logger = logging.NewDefaultLogger(os.Stderr)
	}
	c.Logger = logging.NewPipelineLogger(c.Logger, &logging.PipelineLoggerConfig{Buffer: true})
	c.Retry = c.Retry.WithDefaults()
	return c
}

func (c *PostgresDatastoreConfig) Validate() error {
	if !schemaNamePattern.MatchString(c.Schema) {
		return fmt.Errorf("Schema must be a lowercase unquoted identifier, got %q", c.Schema)
	}
	if err := c.Retry.Validate(); err != nil {
		return fmt.Errorf("Retry: %w", err)
	}
	return nil
}
