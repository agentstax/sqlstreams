package scenarios

import (
	"fmt"
	"time"

	"github.com/agentstax/sqlstreams/.bench/scenario"
	"github.com/agentstax/sqlstreams/pkg/stream"
)

// The multistream family: one deployment shape over one, four, and sixteen
// streams, driven as hard as the producers can, so the achieved rate is the
// ceiling and the server columns say what set it.
var (
	MultistreamUnpaced1  = multistreamUnpaced(1)
	MultistreamUnpaced4  = multistreamUnpaced(4)
	MultistreamUnpaced16 = multistreamUnpaced(16)
)

// multistreamUnpaced is the ceiling search over a stream count: two groups
// on every stream, explicit batches from four callers per stream, a pool
// wide enough that no caller waits for a connection, and aggregate
// counters instead of per-message records so recording never sets the
// ceiling. Backlog and headroom are declared report: an unpaced hold
// outrunning its consumers is a finding, not a failure.
func multistreamUnpaced(streams int) *scenario.Scenario {
	declared := make([]scenario.StreamDeclaration, 0, streams)
	for i := 1; i <= streams; i++ {
		declared = append(declared, scenario.StreamDeclaration{
			Name:            fmt.Sprintf("orders-%d", i),
			DeliveryLogMode: stream.DeliveryLogModeAll,
			Groups: []scenario.GroupDeclaration{
				{Name: "fraud-scoring", MaxRetries: 3, BatchLimit: 2000, QueueSize: 8000, MessageConcurrency: 4, ClaimPollRate: 100 * time.Millisecond},
				{Name: "analytics", MaxRetries: 3, BatchLimit: 2000, QueueSize: 8000, MessageConcurrency: 4, ClaimPollRate: 100 * time.Millisecond},
			},
		})
	}
	return &scenario.Scenario{
		DisableMessageRecording: true,
		Name:                    fmt.Sprintf("multistream-unpaced-%d", streams),
		Summary:                 fmt.Sprintf("the ceiling over %d streams: unpaced explicit batches from four callers per stream, two groups on every stream", streams),
		Duration:                150 * time.Second,
		Streams:                 declared,
		Producer: []scenario.ProducerPhase{
			{Name: "warm", Warmup: true, Unpaced: true, Duration: 30 * time.Second},
			{Name: "hold", Unpaced: true, Duration: 2 * time.Minute},
		},
		Consumers: []scenario.ConsumerChange{{At: 0, Instances: 2}},
		Expect: []scenario.Expectation{
			{Check: scenario.CheckLost, Want: scenario.WantZero},
			{Check: scenario.CheckUnexpected, Want: scenario.WantZero},
			{Check: scenario.CheckRecovered, Want: scenario.WantReport},
			{Check: scenario.CheckUndelivered, Want: scenario.WantZero},
			{Check: scenario.CheckDuplicates, Want: scenario.WantReport},
			{Check: scenario.CheckUnbucketed, Want: scenario.WantZero},
			{Check: scenario.CheckReclaims, Want: scenario.WantReport},
			{Check: scenario.CheckDead, Want: scenario.WantZero},
			{Check: scenario.CheckBacklogBounded, Want: scenario.WantReport},
			{Check: scenario.CheckGeneratorHeadroom, Want: scenario.WantReport},
			{Check: scenario.CheckErrors, Want: scenario.WantZero},
		},
		DisableExceptionConsumers: true, ExplicitBatching: true, ProducerConcurrency: 4,
		ProducerBatchSize: 250, ProducerBatchConcurrency: 4, PayloadBytes: 1000, MaxConns: 64,
	}
}
