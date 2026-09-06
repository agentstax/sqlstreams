package partitioncount

import (
	"github.com/agentstax/vulkan/pkg/alert"
	alertcontroller "github.com/agentstax/vulkan/pkg/alert/controller"
)

var JobName = "alert." + alert.AlertPartitionCount.Name

// NewJob builds the schedule the partition_count alert is evaluated on.
// cfg may be nil or sparse.
func NewJob(cfg *alert.PartitionCountAlertConfig) (*alertcontroller.Job, error) {
	if cfg == nil {
		cfg = &alert.PartitionCountAlertConfig{}
	}
	cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	data, err := alert.NewJobPayload(cfg.Threshold)
	if err != nil {
		return nil, err
	}

	// exclusive so runs never overlap
	return alertcontroller.NewJob(JobName, cfg.ScheduleExpression, data)
}
