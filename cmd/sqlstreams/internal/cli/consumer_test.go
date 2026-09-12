package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestResourceCommandPaths(t *testing.T) {
	for _, path := range [][]string{
		{"consumer", "worker", "list"},
		{"consumer", "binding", "get"},
		{"system", "binding", "list"},
		{"consumer", "destroy"},
	} {
		t.Run(strings.Join(path, "/"), func(t *testing.T) {
			root, _ := newRootCmd()
			command, remaining, err := root.Find(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(remaining) != 0 || command.CommandPath() != "sqlstreams "+strings.Join(path, " ") {
				t.Fatalf("resolved %q with remaining arguments %v", command.CommandPath(), remaining)
			}
		})
	}
}

func TestAlertConsumerSelectorRequiresStream(t *testing.T) {
	for _, verb := range []string{"latest", "history", "list"} {
		t.Run(verb, func(t *testing.T) {
			root, _ := newRootCmd()
			var output bytes.Buffer
			root.SetOut(&output)
			root.SetErr(&output)
			args := []string{"alert", verb}
			if verb != "list" {
				args = append(args, "partition_count")
			}
			root.SetArgs(append(args, "--consumer", "billing"))
			err := root.Execute()
			if err == nil || !strings.Contains(err.Error(), "--consumer requires --stream") {
				t.Fatalf("expected consumer selector validation before database access, got %v", err)
			}
		})
	}
}
