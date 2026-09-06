package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestConsumerCommandPaths(t *testing.T) {
	for _, path := range [][]string{
		{"consumer", "config", "get"},
		{"consumer", "binding", "get"},
		{"consumer", "binding", "list"},
		{"consumer", "destroy"},
	} {
		t.Run(strings.Join(path, "/"), func(t *testing.T) {
			root, _ := newRootCmd()
			command, remaining, err := root.Find(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(remaining) != 0 || command.CommandPath() != "vulkan "+strings.Join(path, " ") {
				t.Fatalf("resolved %q with remaining arguments %v", command.CommandPath(), remaining)
			}
		})
	}
}

func TestAlertConsumerSelectorRequiresTopic(t *testing.T) {
	for _, verb := range []string{"get", "list"} {
		t.Run(verb, func(t *testing.T) {
			root, _ := newRootCmd()
			var output bytes.Buffer
			root.SetOut(&output)
			root.SetErr(&output)
			args := []string{"alert", verb}
			if verb == "get" {
				args = append(args, "disk_pressure")
			}
			root.SetArgs(append(args, "--consumer", "billing"))
			err := root.Execute()
			if err == nil || !strings.Contains(err.Error(), "--consumer requires --topic") {
				t.Fatalf("expected consumer selector validation before database access, got %v", err)
			}
		})
	}
}
