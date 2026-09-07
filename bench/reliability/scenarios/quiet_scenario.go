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
	// its own, so both are expected at zero; a slipped schedule would mean
	// the numbers measured the generator
	Expect: []scenario.Expectation{
		{Check: scenario.CheckLost, Want: scenario.WantZero},
		{Check: scenario.CheckUnexpected, Want: scenario.WantZero},
		{Check: scenario.CheckRecovered, Want: scenario.WantReport},
		{Check: scenario.CheckUndelivered, Want: scenario.WantZero},
		{Check: scenario.CheckDuplicates, Want: scenario.WantReport},
		{Check: scenario.CheckUnbucketed, Want: scenario.WantZero},
		{Check: scenario.CheckReclaims, Want: scenario.WantZero},
		{Check: scenario.CheckDead, Want: scenario.WantZero},
		{Check: scenario.CheckScheduleKept, Want: scenario.WantZero},
	},
}
