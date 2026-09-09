package scenarios

import (
	"time"

	"github.com/agentstax/sqlstreams/.bench/reliability/scenario"
)

var Dev = &scenario.Scenario{
	Name:     "dev",
	Summary:  "the quiet run at one minute, for a laptop",
	Duration: time.Minute,
	Streams:  Quiet.Streams,
	Producer: []scenario.ProducerPhase{
		{Name: "hold", Rate: 200, Duration: time.Minute},
	},
	Consumers: []scenario.ConsumerChange{
		{At: 0, Instances: 3},
	},
	Expect: Quiet.Expect,
}
