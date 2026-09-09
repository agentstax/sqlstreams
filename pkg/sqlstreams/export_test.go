package sqlstreams

import "github.com/agentstax/sqlstreams/pkg/datastore"

// The external test package reads what the client captured at construction.

func (c *Client) Datastore() *datastore.PostgresDatastore { return c.ds }

func (c *Client) ManagerDisabled() bool { return c.disableManager }
