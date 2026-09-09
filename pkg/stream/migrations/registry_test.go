package migrations

import (
	"testing"

	"github.com/agentstax/sqlstreams/pkg/migrate"
)

// invariant: the shipped stream registry passes Validate, so a gapped or
// reordered step fails here and never reaches a migrate command.
func TestStreamRegistryIsValid(t *testing.T) {
	if err := migrate.Validate(Registry); err != nil {
		t.Fatalf("Validate(Registry) = %v, want nil", err)
	}
}
