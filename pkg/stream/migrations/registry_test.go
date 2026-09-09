package migrations

import (
	"testing"

	"github.com/agentstax/sqlstreams/pkg/migrate"
)

// The real registry must always be valid, so an out-of-order or gapped step
// added later fails the build's tests, not production.
func TestRegistryValid(t *testing.T) {
	if err := migrate.Validate(Registry); err != nil {
		t.Fatalf("stream migrations registry invalid: %v", err)
	}
}
