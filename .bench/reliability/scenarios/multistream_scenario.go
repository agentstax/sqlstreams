package scenarios

import (
	"fmt"
	"time"

	"github.com/agentstax/sqlstreams/.bench/reliability/scenario"
	"github.com/agentstax/sqlstreams/pkg/stream"
)

// The multistream ladders: the same total load stepped over one, four, and
// sixteen streams, so the stream counts compare at equal rungs.
var (
	Multistream1  = multistream(1)
	Multistream4  = multistream(4)
	Multistream16 = multistream(16)
)

// rungTotalRates are the ladder's total produce rates, split evenly over the
// scenario's streams; each rung runs rungDuration.
var rungTotalRates = []int{2000, 4000, 8000, 16000}

const rungDuration = 2 * time.Minute

// multistream is the saturation ladder over a stream count: two groups on
// every stream claiming in batches of 100, the rungs climbing until a guard
// gives. The guards are declared report, since the top rung is meant to
// give; the sustainable rung is read off the phase rows of the verdict.
// Safety still holds at every rung, and the drain after the ladder needs a
// budget to match.
func multistream(streams int) *scenario.Scenario {
	declared := make([]scenario.StreamDeclaration, 0, streams)
	for i := 1; i <= streams; i++ {
		declared = append(declared, scenario.StreamDeclaration{
			Name:            fmt.Sprintf("orders-%d", i),
			DeliveryLogMode: stream.DeliveryLogModeAll,
			Groups: []scenario.GroupDeclaration{
				{Name: "fraud-scoring", HandlerFailRate: 0, MaxRetries: 3, BatchLimit: 100},
				{Name: "analytics", HandlerFailRate: 0, MaxRetries: 3, BatchLimit: 100},
			},
		})
	}
	phases := make([]scenario.ProducerPhase, 0, len(rungTotalRates))
	for _, total := range rungTotalRates {
		phases = append(phases, scenario.ProducerPhase{Name: fmt.Sprintf("rung-%d", total), Rate: total / streams, Duration: rungDuration})
	}
	return &scenario.Scenario{
		Name:      fmt.Sprintf("multistream-%d", streams),
		Summary:   fmt.Sprintf("the saturation ladder over %d streams: the same total rate stepped up per rung until a guard gives, two groups on every stream", streams),
		Duration:  rungDuration * time.Duration(len(rungTotalRates)),
		Streams:   declared,
		Producer:  phases,
		Consumers: []scenario.ConsumerChange{{At: 0, Instances: 2}},
		// four batch workers per stream commit about seventy batches a second
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
