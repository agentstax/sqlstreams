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

// closed set: the gate admits a build iff
// minCompatibleVersion <= buildVersion <= version -- a schema behind the
// build is older, one whose floor is past the build is newer, and a schema
// migrated past the build by additive steps alone stays supported.
func TestClassifySchemaSupportAdmitsABuildInsideTheCompatibilityWindow(t *testing.T) {
	tests := []struct {
		name          string
		version       int64
		minCompatible int64
		build         int64
		want          SchemaSupport
	}{
		{name: "same version", version: 3, minCompatible: 0, build: 3, want: SchemaSupported},
		{name: "schema behind the build", version: 2, minCompatible: 0, build: 3, want: SchemaOlderThanBuild},
		{name: "additive steps past the build", version: 5, minCompatible: 0, build: 3, want: SchemaSupported},
		{name: "breaking step at the build", version: 5, minCompatible: 3, build: 3, want: SchemaSupported},
		{name: "breaking step past the build", version: 5, minCompatible: 4, build: 3, want: SchemaNewerThanBuild},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ClassifySchemaSupport(test.version, test.minCompatible, test.build)
			if got != test.want {
				t.Fatalf("ClassifySchemaSupport(version %d, minCompatible %d, build %d) = %s, want %s", test.version, test.minCompatible, test.build, got, test.want)
			}
		})
	}
}
