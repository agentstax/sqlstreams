package admin

type MessageAdminConfig struct {
	// AllowDestroy - whether this admin may destroy streams at all.
	// Default: false.
	//
	// A service that only ever registers streams should never opt in --
	// create is recoverable, destroy is not.
	AllowDestroy bool
}

func (c *MessageAdminConfig) WithDefaults() *MessageAdminConfig {
	return c
}

func (c *MessageAdminConfig) Validate() error {
	return nil
}
