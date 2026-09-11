package scenarios

import (
	"time"

	"github.com/agentstax/sqlstreams/.bench/scenario"
	"github.com/agentstax/sqlstreams/pkg/stream"
)

var Quiet = &scenario.Scenario{
	Name:     "quiet",
	Summary:  "the hour-long quiet run: constant load, fixed consumers, no chaos, nothing may move",
	Duration: 60 * time.Minute,
	Streams: []scenario.StreamDeclaration{{
		Name:            "orders",
		DeliveryLogMode: stream.DeliveryLogModeAll,
		Groups:          []scenario.GroupDeclaration{{Name: "fraud-scoring", HandlerFailRate: 0, MaxRetries: 3}},
	}},
	Producer: []scenario.ProducerPhase{
		{Name: "hold", Rate: 200, Duration: 60 * time.Minute},
	},
	Consumers: []scenario.ConsumerChange{
		{At: 0, Instances: 3},
	},
	// under fail rate 0 and no chaos a reclaim or a dead row is a finding on
	// its own, so both are expected at zero; a slipped schedule, a growing
	// backlog, or a starved container would mean the numbers measured the
	// generator, not the library
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
		{Check: scenario.CheckBacklogBounded, Want: scenario.WantZero},
		{Check: scenario.CheckGeneratorHeadroom, Want: scenario.WantZero},
	},
}
