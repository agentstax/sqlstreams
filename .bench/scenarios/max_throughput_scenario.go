package scenarios

import (
	"time"

	"github.com/allegedlyreliable/sqlstreams/.bench/scenario"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
)

var MaxThroughput = &scenario.Scenario{
	DisableMessageRecording: true,
	Name:                    "max-throughput",
	Summary:                 "1 KB explicit batches, unpaced production, one group, retention and key vacuum enabled",
	Duration:                30 * time.Minute,
	Streams: []scenario.StreamDeclaration{{
		Name: "orders", DeliveryLogMode: stream.DeliveryLogModeFailures,
		PartitionSize: 1_000_000, RetentionTTL: 120 * time.Second, IdempotencyKeyTTL: 120 * time.Second,
		Janitor: &stream.JanitorConfig{PollRate: time.Second, SweepBatchSize: 10_000,
			PartialSweepGracePeriod: 30 * time.Second, CleanupTimeout: 30 * time.Second},
		Vacuum:        &stream.VacuumConfig{PollRate: 120 * time.Second, VacuumTimeout: 60 * time.Second},
		VacuumEnabled: true,
		Groups: []scenario.GroupDeclaration{{Name: "processor", BatchLimit: 16_000,
			QueueSize: 64_000, MessageConcurrency: 4, ClaimPollRate: 100 * time.Millisecond}},
	}},
	Producer: []scenario.ProducerPhase{
		{Name: "warm", Warmup: true, Unpaced: true, Duration: 5 * time.Minute},
		{Name: "hold", Unpaced: true, Duration: 25 * time.Minute},
	},
	Consumers: []scenario.ConsumerChange{{At: 0, Instances: 1}},
	Expect: []scenario.Expectation{
		{Check: scenario.CheckLost, Want: scenario.WantZero},
		{Check: scenario.CheckUnexpected, Want: scenario.WantZero},
		{Check: scenario.CheckRecovered, Want: scenario.WantReport},
		{Check: scenario.CheckUndelivered, Want: scenario.WantZero},
		{Check: scenario.CheckDuplicates, Want: scenario.WantZero},
		{Check: scenario.CheckUnbucketed, Want: scenario.WantZero},
		{Check: scenario.CheckReclaims, Want: scenario.WantZero},
		{Check: scenario.CheckDead, Want: scenario.WantZero},
		{Check: scenario.CheckBacklogBounded, Want: scenario.WantZero},
		{Check: scenario.CheckGeneratorHeadroom, Want: scenario.WantReport},
		{Check: scenario.CheckErrors, Want: scenario.WantZero},
	},
	DisableExceptionConsumers: true, ExplicitBatching: true, ProducerConcurrency: 4,
	ProducerBatchSize: 250, ProducerBatchConcurrency: 4, PayloadBytes: 1000, MaxConns: 8,
}
