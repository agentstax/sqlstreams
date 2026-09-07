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
		Expect:          []Expectation{{Check: CheckLost, Want: "0"}},
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
	scenario.Expect = []Expectation{
		{Check: CheckLost, Want: "0"},
		{Check: CheckLost, Want: "0"},
	}
	if err := scenario.Validate(); err == nil {
		t.Fatal("Validate accepted the same check declared twice")
	}
}

func TestValidateRejectsAConsumerChangeAfterTheRun(t *testing.T) {
	scenario := validScenario()
	scenario.Consumers = append(scenario.Consumers, ConsumerChange{At: 2 * time.Minute, Instances: 0})
	if err := scenario.Validate(); err == nil {
		t.Fatal("Validate accepted a consumer change at 2m in a 1m run")
	}
}
