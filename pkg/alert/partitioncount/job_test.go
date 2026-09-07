package partitioncount

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/agentstax/vulkan/pkg/alert"
)

func TestJobCarriesPendingPolicy(t *testing.T) {
	job, err := NewJob(&alert.PartitionCountAlertConfig{
		Threshold:       100,
		PendingDuration: 3 * time.Minute, MaximumGap: time.Minute, DisablePending: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(job.Payload)
	if err != nil {
		t.Fatal(err)
	}
	var decoded alert.JobPayload
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := decoded.WithDefaults().Validate(); err != nil {
		t.Fatal(err)
	}
	if decoded.Threshold != 100 || decoded.PendingDuration != 3*time.Minute || decoded.MaximumGap != time.Minute || !decoded.DisablePending {
		t.Fatalf("consumed policy = %+v", decoded)
	}
	if job.Cron != "@every 1m" {
		t.Fatalf("cadence changed: %q", job.Cron)
	}
}
