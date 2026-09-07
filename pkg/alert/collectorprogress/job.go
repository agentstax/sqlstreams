package collectorprogress

import (
	"github.com/agentstax/vulkan/pkg/alert"
	alertcontroller "github.com/agentstax/vulkan/pkg/alert/controller"
)

var JobName = "alert." + alert.AlertMetricsCollectorProgress.Name

// NewJob builds the collector-progress schedule with its consumed timing policy.
func NewJob(cfg *alert.MetricsCollectorProgressAlertConfig) (*alertcontroller.Job, error) {
	if cfg == nil {
		cfg = &alert.MetricsCollectorProgressAlertConfig{}
	}
	cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	data, err := alert.NewJobPayload(0, cfg.PendingDuration, 0, cfg.DisablePending)
	if err != nil {
		return nil, err
	}
	data.MaximumAge = cfg.MaximumAge
	return alertcontroller.NewJob(JobName, cfg.ScheduleExpression, data)
}
