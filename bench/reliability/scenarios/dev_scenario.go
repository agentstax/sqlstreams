package scenarios

import (
	"time"

	"github.com/agentstax/vulkan/bench/reliability/scenario"
	"github.com/agentstax/vulkan/pkg/topic"
)

var Dev = &scenario.Scenario{
	Name:            "dev",
	Summary:         "the quiet run at one minute, for a laptop",
	Topic:           "orders",
	DeliveryLogMode: topic.DeliveryLogModeAll,
	Group:           "fraud-scoring",
	HandlerFailRate: 0,
	MaxRetries:      3,
	Duration:        time.Minute,
	Producer: []scenario.ProducerPhase{
		{Name: "hold", Rate: 200, Duration: time.Minute},
	},
	Consumers: []scenario.ConsumerChange{
		{At: 0, Instances: 3},
	},
	Expect: Quiet.Expect,
}
