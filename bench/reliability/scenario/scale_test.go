package scenario

import (
	"testing"
	"time"
)

func TestScaledShortensEveryDurationAndOffset(t *testing.T) {
	declared := validScenario()
	declared.Duration = 60 * time.Minute
	declared.Producer = []ProducerPhase{
		{Name: "warm", Rate: 200, Duration: 20 * time.Minute},
		{Name: "hold", Rate: 200, Duration: 40 * time.Minute},
	}
	declared.Consumers = []ConsumerChange{{At: 0, Instances: 3}, {At: 30 * time.Minute, Instances: 8}}

	scaled := declared.Scaled(1.0 / 60)
	if scaled.Duration != time.Minute {
		t.Fatalf("Duration scaled to %v, want 1m", scaled.Duration)
	}
	if scaled.Producer[0].Duration != 20*time.Second || scaled.Producer[1].Duration != 40*time.Second {
		t.Fatalf("phases scaled to %v and %v, want 20s and 40s", scaled.Producer[0].Duration, scaled.Producer[1].Duration)
	}
	if scaled.Consumers[1].At != 30*time.Second {
		t.Fatalf("consumer change scaled to %v, want 30s", scaled.Consumers[1].At)
	}
	if err := scaled.Validate(); err != nil {
		t.Fatalf("scaled scenario does not validate: %v", err)
	}
	if declared.Duration != 60*time.Minute || declared.Producer[0].Duration != 20*time.Minute {
		t.Fatal("Scaled mutated the original")
	}
}
