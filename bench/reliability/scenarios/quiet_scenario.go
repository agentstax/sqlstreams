package scenarios

import (
	"time"

	"github.com/agentstax/vulkan/bench/reliability/scenario"
	"github.com/agentstax/vulkan/pkg/topic"
)

var Quiet = &scenario.Scenario{
	Name:            "quiet",
	Summary:         "the hour-long quiet run: constant load, fixed consumers, no chaos, nothing may move",
	Topic:           "orders",
	DeliveryLogMode: topic.DeliveryLogModeAll,
	Group:           "fraud-scoring",
	HandlerFailRate: 0,
	MaxRetries:      3,
	Duration:        60 * time.Minute,
	Producer: []scenario.ProducerPhase{
		{Name: "hold", Rate: 200, Duration: 60 * time.Minute},
	},
	Consumers: []scenario.ConsumerChange{
		{At: 0, Instances: 3},
	},
	// under fail rate 0 and no chaos a reclaim or a dead row is a finding on
	// its own, so both are expected at zero
	Expect: []scenario.Expectation{
		{Check: scenario.CheckLost, Want: "0"},
		{Check: scenario.CheckUndelivered, Want: "0"},
		{Check: scenario.CheckUnbucketed, Want: "0"},
		{Check: scenario.CheckDuplicates, Want: "report"},
		{Check: scenario.CheckReclaims, Want: "0"},
		{Check: scenario.CheckDead, Want: "0"},
	},
}
