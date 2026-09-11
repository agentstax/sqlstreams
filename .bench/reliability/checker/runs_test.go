package checker

import "testing"

func TestDifferentExecutionEnvironmentsDoNotShareAResultIdentity(t *testing.T) {
	original := &Verdict{Fingerprint: &Fingerprint{LibrarySha: "same-library", Execution: "native", Runtime: map[string]string{"GOMAXPROCS": "4"}}}
	for _, changed := range []*Fingerprint{
		{LibrarySha: "same-library", Execution: "compose", Runtime: map[string]string{"GOMAXPROCS": "4"}},
		{LibrarySha: "same-library", Execution: "native", Runtime: map[string]string{"GOMAXPROCS": "8"}},
		{LibrarySha: "same-library", Execution: "native", Runtime: map[string]string{"GOMAXPROCS": "4"}, BinarySha: "different-binary"},
	} {
		if identityKey(original) == identityKey(&Verdict{Fingerprint: changed}) {
			t.Fatal("different environments grouped together")
		}
	}
}
