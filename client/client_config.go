package sqlstreams

import (
	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/common/logging"
	"github.com/agentstax/sqlstreams/pkg/datastore"
)

// ClientConfig supplies construction-time settings. NewClient captures values
// and copies Retry; later edits do not reconfigure the client. Logger is shared.
type ClientConfig struct {
	// Schema - the Postgres namespace holding every sqlstreams table.
	// Default: "sqlstreams".
	//
	// One schema is one installation: two clients on two schemas in the
	// same database share nothing.
	Schema string

	// AllowDestroy - whether this client may destroy streams at all.
	// Default: false.
	//
	// A service that only ever registers streams should never opt in --
	// create is recoverable, destroy is not.
	AllowDestroy bool

	// DisableManager - whether Consume skips running the system manager
	// beside its session; explicit Manager().Run calls are unaffected.
	// Default: false.
	//
	// Set it where upkeep must not run in this process: a deployment with
	// dedicated `sqlstreams manager run` processes, or consumers under a
	// database role without DDL rights (the stream janitor runs DDL).
	DisableManager bool

	// Logger - your own *slog.Logger or anything satisfying logging.Logger.
	// Held once on the datastore NewClient builds; no config below the
	// client carries one.
	// Default: text lines to stderr, warn level and up.
	Logger logging.Logger

	// Retry - transient-error retry policy for every Postgres call the
	// client makes, never a message's redelivery. Held once, like Logger.
	// Default: common.NewDefaultRetryPolicy().
	Retry *common.RetryPolicy
}

// WithDefaults fills Schema; Logger and Retry resolve in
// PostgresDatastoreConfig, their single owner.
func (c *ClientConfig) WithDefaults() *ClientConfig {
	if c.Schema == "" {
		c.Schema = datastore.DefaultSchema
	}
	return c
}

func (c *ClientConfig) Validate() error {
	return nil
}
