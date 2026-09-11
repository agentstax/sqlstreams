package scenarios

import (
	"os"
	"testing"
)

// Every declared scenario validates, and its printed form matches the
// .scenario file columns it byte for byte. A stale file fails here; the fix is
// to regenerate it with `go run . -scenario <name> > scenarios/<name>.scenario`
// from .bench after reviewing the diff.
func TestDeclarationsMatchTheirFiles(t *testing.T) {
	for _, declared := range All {
		t.Run(declared.Name, func(t *testing.T) {
			if err := declared.Validate(); err != nil {
				t.Fatalf("Validate: %v", err)
			}
			path := declared.Name + ".scenario"
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			got := declared.String()
			if got != string(want) {
				t.Errorf("%s is stale\n--- printed ---\n%s--- file ---\n%s", path, got, want)
			}
		})
	}
}

func TestByName(t *testing.T) {
	if _, ok := ByName("quiet"); !ok {
		t.Fatal("quiet scenario is not declared")
	}
	if _, ok := ByName("nope"); ok {
		t.Fatal("ByName found a scenario that does not exist")
	}
}
