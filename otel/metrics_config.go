package otel

import (
	"fmt"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/common/logging"
	"github.com/allegedlyreliable/sqlstreams/pkg/datastore"
)

type MetricsConfig struct {
	// Schema selects the SQLStreams installation. Default: "sqlstreams".
	Schema string

	// CollectTimeout bounds each collection's Postgres reads.
	// Default: 5s.
	CollectTimeout time.Duration

	// Logger and Retry resolve in the datastore constructed over the caller's pool.
	// Defaults: warn-level stderr logging and the default datastore retry policy.
	Logger logging.Logger
	Retry  *common.RetryPolicy
}

func (c *MetricsConfig) WithDefaults() *MetricsConfig {
	if c.Schema == "" {
		c.Schema = datastore.DefaultSchema
	}
	if c.CollectTimeout == 0 {
		c.CollectTimeout = 5 * time.Second
	}
	return c
}

// Validate runs after WithDefaults -- anything still out of range here was
// set by the caller, not left unset.
func (c *MetricsConfig) Validate() error {
	if c.CollectTimeout <= 0 {
		return fmt.Errorf("CollectTimeout must be > 0, got %v", c.CollectTimeout)
	}
	return nil
}
