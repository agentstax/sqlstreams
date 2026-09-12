package scenarios

import (
	"fmt"
	"time"

	"github.com/agentstax/sqlstreams/.bench/scenario"
	"github.com/agentstax/sqlstreams/pkg/stream"
)

// The idle-fleet family: a deployment with nothing to do over 16, 160, and
// 1600 streams, so every statement the server runs is upkeep -- manager
// ticks, claim attempts, heartbeats, sweeps, and empty consumer polls. The
// statement column of the report says which verb pays for the fleet's size.
var (
	IdleFleet16   = idleFleet(16, 5*time.Minute)
	IdleFleet160  = idleFleet(160, 5*time.Minute)
	IdleFleet1600 = idleFleet(1600, 10*time.Minute)
)

// idleFleet declares one group on every stream with the library's defaults,
// so the idle cost measured is the default deployment's. The settle warmup
// absorbs every role registering the streams -- 160 streams register in
// under a minute across four roles, so the largest cell gets twice the
// time -- and the ten-minute hold is the measured window. A replica runs
// one session per group, so worker rows and sessions both scale with the
// stream count and `-replicas`.
func idleFleet(streams int, settle time.Duration) *scenario.Scenario {
	declared := make([]scenario.StreamDeclaration, 0, streams)
	for i := 1; i <= streams; i++ {
		declared = append(declared, scenario.StreamDeclaration{
			Name:            fmt.Sprintf("orders-%d", i),
			DeliveryLogMode: stream.DeliveryLogModeFailures,
			Groups:          []scenario.GroupDeclaration{{Name: "fraud-scoring", MaxRetries: 3}},
		})
	}
	return &scenario.Scenario{
		Name:     fmt.Sprintf("idle-fleet-%d", streams),
		Summary:  fmt.Sprintf("the idle fleet over %d streams: nothing produced, one group on every stream, every replica running a session on each", streams),
		Duration: settle + 10*time.Minute,
		Streams:  declared,
		Producer: []scenario.ProducerPhase{
			{Name: "settle", Warmup: true, Duration: settle},
			{Name: "idle", Duration: 10 * time.Minute},
		},
		Consumers: []scenario.ConsumerChange{{At: 0, Instances: 1}},
		// nothing is produced, so the identity checks hold trivially; a
		// consumer container busy polling is the finding, not a failure
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
		},
		MaxConns: 64,
	}
}
