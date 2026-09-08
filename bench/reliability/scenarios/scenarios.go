package scenarios

// The declared scenarios, one file each (<name>_scenario.go); the .scenario
// file columns each is its printed form, kept in step by the test here.

import (
	"strings"

	"github.com/agentstax/vulkan/bench/reliability/scenario"
)

var All = []*scenario.Scenario{Quiet, Dev, Multitopic1, Multitopic4, Multitopic16, Throughput}

func ByName(name string) (*scenario.Scenario, bool) {
	for _, declared := range All {
		if declared.Name == name {
			return declared, true
		}
	}
	return nil, false
}

func Names() string {
	names := make([]string, 0, len(All))
	for _, declared := range All {
		names = append(names, declared.Name)
	}
	return strings.Join(names, ", ")
}
