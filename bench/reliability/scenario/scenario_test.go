package scenario

import (
	"testing"
	"time"

	"github.com/agentstax/vulkan/pkg/topic"
)

func validScenario() Scenario {
	return Scenario{
		Name:            "test",
		Summary:         "a valid scenario for the validation tests",
		Topic:           "orders",
		DeliveryLogMode: topic.DeliveryLogModeAll,
		Group:           "fraud-scoring",
		MaxRetries:      3,
		Duration:        time.Minute,
		Producer:        []ProducerPhase{{Name: "hold", Rate: 200, Duration: time.Minute}},
		Consumers:       []ConsumerChange{{At: 0, Instances: 3}},
		Expect:          append([]Expectation{}, Invariants...),
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
		t.Fatal("Validate accepted a timeline ending with no consumer to drain the topic")
	}
}

func TestValidateRejectsAConsumerChangeAfterTheRun(t *testing.T) {
	scenario := validScenario()
	scenario.Consumers = append(scenario.Consumers, ConsumerChange{At: 2 * time.Minute, Instances: 0})
	if err := scenario.Validate(); err == nil {
		t.Fatal("Validate accepted a consumer change at 2m in a 1m run")
	}
}
