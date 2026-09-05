package admin

type MessageAdminConfig struct {
	// AllowDestroy - whether this admin may destroy topics at all.
	// Default: false.
	//
	// A service that only ever registers topics should never opt in --
	// create is recoverable, destroy is not.
	AllowDestroy bool
}

func (c *MessageAdminConfig) WithDefaults() *MessageAdminConfig {
	return c
}

// Validate runs after WithDefaults -- anything still out of range here was
// set by the caller, not left unset.
func (c *MessageAdminConfig) Validate() error {
	return nil
}
