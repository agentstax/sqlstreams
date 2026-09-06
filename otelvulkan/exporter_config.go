package otelvulkan

import (
	"fmt"
	"time"

	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/common/logging"
	"github.com/agentstax/vulkan/pkg/datastore"
)

type ExporterConfig struct {
	// Schema selects the Vulkan installation. Default: "vulkan".
	Schema string

	// CollectTimeout bounds the Postgres reads a scrape drives -- the
	// instrument registration pass and the observation callback. A scrape
	// request may carry no deadline of its own.
	// Default: 5s.
	CollectTimeout time.Duration

	// Logger and Retry resolve in the datastore constructed over the caller's pool.
	// Defaults: warn-level stderr logging and the default datastore retry policy.
	Logger logging.Logger
	Retry  *common.RetryPolicy
}

func (c *ExporterConfig) WithDefaults() *ExporterConfig {
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
func (c *ExporterConfig) Validate() error {
	if c.CollectTimeout <= 0 {
		return fmt.Errorf("CollectTimeout must be > 0, got %v", c.CollectTimeout)
	}
	return nil
}
