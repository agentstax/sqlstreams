package otelvulkan

import (
	"errors"
	"fmt"
	"time"

	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/common/logging"
	"github.com/agentstax/vulkan/pkg/datastore"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

type MetricsConfig struct {
	// Schema selects the Vulkan installation. Default: "vulkan".
	Schema string

	// Meter receives the metric instruments -- pass one from your own
	// provider to feed your own otel pipeline.
	// Default: the global otel provider's meter.
	Meter metric.Meter

	// CollectTimeout bounds the Postgres reads a collection drives -- the
	// instrument registration pass and the observation callback. A
	// collection may carry no deadline of its own.
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
	if c.Meter == nil {
		c.Meter = otel.GetMeterProvider().Meter(meterScopeName)
	}
	if c.CollectTimeout == 0 {
		c.CollectTimeout = 5 * time.Second
	}
	return c
}

// Validate runs after WithDefaults -- anything still out of range here was
// set by the caller, not left unset.
func (c *MetricsConfig) Validate() error {
	if c.Meter == nil {
		return errors.New("Meter must not be nil")
	}
	if c.CollectTimeout <= 0 {
		return fmt.Errorf("CollectTimeout must be > 0, got %v", c.CollectTimeout)
	}
	return nil
}
