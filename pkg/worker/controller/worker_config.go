package controller

type WorkerConfig struct {
	// Metadata - the worker kind's config, stored as JSONB.
	// Default: nil (stored as '{}').
	Metadata any
}

func (c *WorkerConfig) WithDefaults() *WorkerConfig {
	return c
}

func (c *WorkerConfig) Validate() error {
	return nil
}
