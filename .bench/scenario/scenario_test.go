package scenario

import (
	"testing"
	"time"

	"github.com/agentstax/sqlstreams/pkg/stream"
)

func validScenario() Scenario {
	return Scenario{
		Name:     "test",
		Summary:  "a valid scenario for the validation tests",
		Duration: time.Minute,
		Streams: []StreamDeclaration{{
			Name:            "orders",
			DeliveryLogMode: stream.DeliveryLogModeAll,
			Groups:          []GroupDeclaration{{Name: "fraud-scoring", MaxRetries: 3}},
		}},
		Producer:  []ProducerPhase{{Name: "hold", Rate: 200, Duration: time.Minute}},
		Consumers: []ConsumerChange{{At: 0, Instances: 3}},
		Expect:    append([]Expectation{}, Invariants...),
	}
}

func TestValidateRejectsARepeatedStreamOrGroup(t *testing.T) {
	scenario := validScenario()
	scenario.Streams = append(scenario.Streams, scenario.Streams[0])
	if err := scenario.Validate(); err == nil {
		t.Fatal("Validate accepted the same stream declared twice")
	}

	scenario = validScenario()
	scenario.Streams[0].Groups = append(scenario.Streams[0].Groups, scenario.Streams[0].Groups[0])
	if err := scenario.Validate(); err == nil {
		t.Fatal("Validate accepted the same group declared twice on one stream")
	}
}

func TestValidateAcceptsAValidScenario(t *testing.T) {
	scenario := validScenario()
	if err := scenario.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidateRejectsPhasesThatDoNotSumToDuration(t *testing.T) {
	scenario := validScenario()
	scenario.Producer = []ProducerPhase{{Name: "hold", Rate: 200, Duration: 30 * time.Second}}
	if err := scenario.Validate(); err == nil {
		t.Fatal("Validate accepted phases summing to 30s against a 1m Duration")
	}
}

func TestValidateRejectsARepeatedCheck(t *testing.T) {
	scenario := validScenario()
	scenario.Expect = append(scenario.Expect, Expectation{Check: CheckLost, Want: WantZero})
	if err := scenario.Validate(); err == nil {
		t.Fatal("Validate accepted the same check declared twice")
	}
}

func TestValidateRequiresEveryInvariant(t *testing.T) {
	scenario := validScenario()
	scenario.Expect = scenario.Expect[1:]
	if err := scenario.Validate(); err == nil {
		t.Fatal("Validate accepted a scenario that dropped the lost check")
	}

	scenario = validScenario()
	scenario.Expect[0].Want = WantReport
	if err := scenario.Validate(); err == nil {
		t.Fatal("Validate accepted lost declared as report")
	}
}

func TestValidateRejectsATimelineEndingAtZeroConsumers(t *testing.T) {
	scenario := validScenario()
	scenario.Consumers = append(scenario.Consumers, ConsumerChange{At: 30 * time.Second, Instances: 0})
	if err := scenario.Validate(); err == nil {
		t.Fatal("Validate accepted a timeline ending with no consumer to drain the stream")
	}
}

func TestValidateRejectsAConsumerChangeAfterTheRun(t *testing.T) {
	scenario := validScenario()
	scenario.Consumers = append(scenario.Consumers, ConsumerChange{At: 2 * time.Minute, Instances: 0})
	if err := scenario.Validate(); err == nil {
		t.Fatal("Validate accepted a consumer change at 2m in a 1m run")
	}
}

func TestUnpacedProductionRequiresAnExplicitConcurrency(t *testing.T) {
	declared := validScenario()
	declared.Producer[0].Unpaced = true
	declared.Producer[0].Rate = 0
	if err := declared.Validate(); err == nil {
		t.Fatal("accepted unbounded caller count")
	}
	declared.ProducerConcurrency = 4
	if err := declared.Validate(); err != nil {
		t.Fatal(err)
	}
	declared.Producer[0].Rate = 200
	if err := declared.Validate(); err == nil {
		t.Fatal("accepted both scheduled and unpaced production")
	}
}

func TestZeroRateRemainsAnIdlePhase(t *testing.T) {
	declared := validScenario()
	declared.Producer[0].Rate = 0
	if err := declared.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := declared.Producer[0].String(); got != "steady 0/s 1m" {
		t.Fatal(got)
	}
}

func TestExplicitBatchesCannotAlsoUseAutomaticBatching(t *testing.T) {
	declared := validScenario()
	declared.Producer[0].Unpaced = true
	declared.Producer[0].Rate = 0
	declared.ProducerConcurrency = 4
	declared.ExplicitBatching = true
	declared.ProducerBatchSize = 250
	if err := declared.Validate(); err != nil {
		t.Fatal(err)
	}
	declared.AutomaticBatching = true
	if err := declared.Validate(); err == nil {
		t.Fatal("accepted competing batch modes")
	}
}

func TestWarmupRequiresAMeasuredPhase(t *testing.T) {
	declared := validScenario()
	declared.Producer[0].Warmup = true
	if err := declared.Validate(); err == nil {
		t.Fatal("accepted a run containing only warmup")
	}
}

func TestReportExpectationsCanBeStrengthenedButZeroCannotBeWeakened(t *testing.T) {
	declared := validScenario()
	for i := range declared.Expect {
		if declared.Expect[i].Check == CheckDuplicates {
			declared.Expect[i].Want = WantZero
		}
	}
	if err := declared.Validate(); err != nil {
		t.Fatal(err)
	}
	for i := range declared.Expect {
		if declared.Expect[i].Check == CheckLost {
			declared.Expect[i].Want = WantReport
		}
	}
	if err := declared.Validate(); err == nil {
		t.Fatal("accepted weakening the loss invariant")
	}
}
