package scenarios

import (
	"fmt"
	"time"

	"github.com/agentstax/vulkan/.bench/reliability/scenario"
	"github.com/agentstax/vulkan/pkg/topic"
)

// The multitopic ladders: the same total load stepped over one, four, and
// sixteen topics, so the topic counts compare at equal rungs.
var (
	Multitopic1  = multitopic(1)
	Multitopic4  = multitopic(4)
	Multitopic16 = multitopic(16)
)

// rungTotalRates are the ladder's total produce rates, split evenly over the
// scenario's topics; each rung runs rungDuration.
var rungTotalRates = []int{2000, 4000, 8000, 16000}

const rungDuration = 2 * time.Minute

// multitopic is the saturation ladder over a topic count: two groups on
// every topic claiming in batches of 100, the rungs climbing until a guard
// gives. The guards are declared report, since the top rung is meant to
// give; the sustainable rung is read off the phase rows of the verdict.
// Safety still holds at every rung, and the drain after the ladder needs a
// budget to match.
func multitopic(topics int) *scenario.Scenario {
	declared := make([]scenario.TopicDeclaration, 0, topics)
	for i := 1; i <= topics; i++ {
		declared = append(declared, scenario.TopicDeclaration{
			Name:            fmt.Sprintf("orders-%d", i),
			DeliveryLogMode: topic.DeliveryLogModeAll,
			Groups: []scenario.GroupDeclaration{
				{Name: "fraud-scoring", HandlerFailRate: 0, MaxRetries: 3, BatchLimit: 100},
				{Name: "analytics", HandlerFailRate: 0, MaxRetries: 3, BatchLimit: 100},
			},
		})
	}
	phases := make([]scenario.ProducerPhase, 0, len(rungTotalRates))
	for _, total := range rungTotalRates {
		phases = append(phases, scenario.ProducerPhase{Name: fmt.Sprintf("rung-%d", total), Rate: total / topics, Duration: rungDuration})
	}
	return &scenario.Scenario{
		Name:      fmt.Sprintf("multitopic-%d", topics),
		Summary:   fmt.Sprintf("the saturation ladder over %d topics: the same total rate stepped up per rung until a guard gives, two groups on every topic", topics),
		Duration:  rungDuration * time.Duration(len(rungTotalRates)),
		Topics:    declared,
		Producer:  phases,
		Consumers: []scenario.ConsumerChange{{At: 0, Instances: 2}},
		// four batch workers per topic commit about seventy batches a second
		// and stall at one Postgres core, so the ladder gives them sixteen
		ProducerBatchConcurrency: 16,
		Expect: []scenario.Expectation{
			{Check: scenario.CheckLost, Want: scenario.WantZero},
			{Check: scenario.CheckUnexpected, Want: scenario.WantZero},
			{Check: scenario.CheckRecovered, Want: scenario.WantReport},
			{Check: scenario.CheckUndelivered, Want: scenario.WantZero},
			{Check: scenario.CheckDuplicates, Want: scenario.WantReport},
			{Check: scenario.CheckUnbucketed, Want: scenario.WantZero},
			{Check: scenario.CheckReclaims, Want: scenario.WantReport},
			{Check: scenario.CheckDead, Want: scenario.WantZero},
			{Check: scenario.CheckScheduleKept, Want: scenario.WantReport},
			{Check: scenario.CheckBacklogBounded, Want: scenario.WantReport},
			{Check: scenario.CheckGeneratorHeadroom, Want: scenario.WantReport},
		},
	}
}
