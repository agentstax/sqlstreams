package scenarios

import (
	"github.com/agentstax/sqlstreams/.bench/scenario"
	"github.com/agentstax/sqlstreams/pkg/stream"
	"time"
)

var Throughput = &scenario.Scenario{
	DisableMessageRecording: true,
	Name:                    "throughput",
	Summary:                 "1 KB messages with automatic batching and one group; short exploration, not a sustainable verdict",
	Duration:                30 * time.Second,
	Streams: []scenario.StreamDeclaration{{Name: "orders", DeliveryLogMode: stream.DeliveryLogModeFailures,
		Groups: []scenario.GroupDeclaration{{Name: "processor", MaxRetries: 3, BatchLimit: 100, ClaimPollRate: 10 * time.Millisecond}},
	}},
	Producer:                 []scenario.ProducerPhase{{Name: "warm", Rate: 2000, Duration: 10 * time.Second}, {Name: "hold", Rate: 2000, Duration: 20 * time.Second}},
	Consumers:                []scenario.ConsumerChange{{At: 0, Instances: 2}},
	Expect:                   append(append([]scenario.Expectation{}, Quiet.Expect...), scenario.Expectation{Check: scenario.CheckErrors, Want: scenario.WantZero}),
	AutomaticBatching:        true,
	PayloadBytes:             1000,
	ProducerBatchSize:        100,
	ProducerBatchConcurrency: 4,
	MaxConns:                 16,
}
