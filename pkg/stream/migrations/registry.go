// Package migrations holds the stream-scope schema migration steps -- one
// migrate.Migration per version, gathered into an explicit ordered Registry.
package migrations

import "github.com/agentstax/sqlstreams/pkg/migrate"

// Registry is the ordered list of stream-scope steps above the v1 baseline
// (createStreamTables builds every stream at version 1, so steps start at version 2).
//
// Add a step by declaring it in its own migration_<version>.go file
// and appending it here -- the slice order is the truth.
// See migrate.Migration for the authoring rules.
var Registry = []migrate.Migration{}

// Version is the stream-scope schema version this build defines.
func Version() int64 {
	return migrate.Version(Registry)
}
