package datastore

// CheckerDatastoreConfig selects aggregate counters when message records are disabled.
type CheckerDatastoreConfig struct{ DisableMessageRecording bool }

func (c *CheckerDatastoreConfig) WithDefaults() *CheckerDatastoreConfig { return c }
func (c *CheckerDatastoreConfig) Validate() error                       { return nil }
