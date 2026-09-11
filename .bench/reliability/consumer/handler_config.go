package consumer

// HandlerConfig selects whether individual handler invocations are recorded.
type HandlerConfig struct {
	// DisableMessageRecording selects aggregate counters. Default: false.
	DisableMessageRecording bool
}

func (c *HandlerConfig) WithDefaults() *HandlerConfig { return c }
func (c *HandlerConfig) Validate() error              { return nil }
