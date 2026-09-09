package migrate

import "testing"

// closed set: the registry shapes Validate accepts -- versions are
// contiguous from 2, so a gapped or reordered step fails the build's tests.
func TestValidateAcceptsOnlyContiguousVersionsFromTwo(t *testing.T) {
	tests := []struct {
		name     string
		versions []int64
		wantErr  bool
	}{
		{name: "empty", versions: nil, wantErr: false},
		{name: "single", versions: []int64{2}, wantErr: false},
		{name: "contiguous", versions: []int64{2, 3, 4}, wantErr: false},
		{name: "starts at 1", versions: []int64{1}, wantErr: true},
		{name: "starts at 3", versions: []int64{3}, wantErr: true},
		{name: "gap", versions: []int64{2, 4}, wantErr: true},
		{name: "duplicate", versions: []int64{2, 3, 3}, wantErr: true},
		{name: "out of order", versions: []int64{3, 2}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registry := make([]Migration, len(test.versions))
			for i, version := range test.versions {
				registry[i] = Migration{Version: version}
			}

			err := Validate(registry)
			if (err != nil) != test.wantErr {
				t.Fatalf("Validate(%v) = %v, want error %v", test.versions, err, test.wantErr)
			}
		})
	}
}

// closed set: MinCompatibleVersion is 0 (additive) or a version no later
// than the step's own.
func TestValidateBoundsMinCompatibleVersionByOwnVersion(t *testing.T) {
	tests := []struct {
		name          string
		minCompatible int64
		wantErr       bool
	}{
		{name: "zero is additive", minCompatible: 0, wantErr: false},
		{name: "below own version", minCompatible: 2, wantErr: false},
		{name: "own version is breaking", minCompatible: 3, wantErr: false},
		{name: "negative", minCompatible: -1, wantErr: true},
		{name: "above own version", minCompatible: 4, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registry := []Migration{
				{Version: 2},
				{Version: 3, MinCompatibleVersion: test.minCompatible},
			}

			err := Validate(registry)
			if (err != nil) != test.wantErr {
				t.Fatalf("Validate(MinCompatibleVersion %d) = %v, want error %v", test.minCompatible, err, test.wantErr)
			}
		})
	}
}
