package workerliveness

import (
	"github.com/agentstax/vulkan/pkg/alert"
	alertcontroller "github.com/agentstax/vulkan/pkg/alert/controller"
)

var JobName = "alert." + alert.AlertWorkerLiveness.Name

// NewJob builds the schedule the worker_liveness alert is evaluated on.
// cfg may be nil or sparse.
func NewJob(cfg *alert.WorkerLivenessAlertConfig) (*alertcontroller.Job, error) {
	if cfg == nil {
		cfg = &alert.WorkerLivenessAlertConfig{}
	}
	cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// the payload's threshold is the shared job shape; Evaluate ignores it
	data, err := alert.NewJobPayload(0)
	if err != nil {
		return nil, err
	}

	// exclusive so runs never overlap
	return alertcontroller.NewJob(JobName, cfg.ScheduleExpression, data)
}
