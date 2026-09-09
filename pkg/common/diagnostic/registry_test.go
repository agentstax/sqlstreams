package diagnostic

import "testing"

// Regression: the three-letter prefix keeps exactly four ASCII digits and rejects old codes.
func TestDiagnosticCodeFormat(t *testing.T) {
	for _, test := range []struct {
		code string
		want bool
	}{
		{"SQL0000", true},
		{"SQL0005", true},
		{"SQL9999", true},
		{"", false},
		{"SQL", false},
		{"SQL005", false},
		{"SQL00005", false},
		{"SQL00a5", false},
		{"SQL０００５", false},
		{"sql0005", false},
		{"SS0005", false},
		{"VK0005", false},
	} {
		t.Run(test.code, func(t *testing.T) {
			if got := isSQLCode(test.code); got != test.want {
				t.Errorf("isSQLCode(%q) = %v, want %v", test.code, got, test.want)
			}
		})
	}
}
