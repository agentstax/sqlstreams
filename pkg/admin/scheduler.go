package admin

import (
	"context"

	"github.com/agentstax/vulkan/pkg/producer"
	"github.com/agentstax/vulkan/pkg/schedule"
	"github.com/agentstax/vulkan/pkg/scheduler"
	"github.com/agentstax/vulkan/pkg/topic"
)

// GetSchedule returns (nil, nil), not an error, if name isn't registered.
func (a *MessageAdmin) GetSchedule(ctx context.Context, name string) (*schedule.Schedule, error) {
	return a.scheduleController.Get(ctx, name)
}

// ListSchedules returns every schedule, ordered by name.
func (a *MessageAdmin) ListSchedules(ctx context.Context) ([]*schedule.Schedule, error) {
	return a.scheduleController.List(ctx)
}

// SuspendSchedule stops the scheduler producing the schedule until unsuspended.
func (a *MessageAdmin) SuspendSchedule(ctx context.Context, name string) error {
	return a.scheduleController.Suspend(ctx, name)
}

// UnsuspendSchedule resumes at the schedule's next scheduled time -- one that
// came due while suspended is dropped, not produced late.
func (a *MessageAdmin) UnsuspendSchedule(ctx context.Context, name string) error {
	return a.scheduleController.Unsuspend(ctx, name)
}

// RunSchedule produces the named schedule's stored message immediately.
// options may be nil for the defaults.
func (a *MessageAdmin) RunSchedule(ctx context.Context, name string, options *scheduler.ScheduleRunOptions) (*producer.ProduceResult[schedule.ScheduleStoredMessage], error) {
	return a.scheduler.RunSchedule(ctx, name, options)
}

// ScheduleStatus is one ScheduleConsumerGroupSummary per consumer group that receives the
// schedule's messages. Counts cover the target topic's retention window.
// Returns ErrScheduleNotFound if name isn't registered.
func (a *MessageAdmin) ScheduleStatus(ctx context.Context, name string) ([]*schedule.ScheduleConsumerGroupSummary, error) {
	found, err := a.scheduleController.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, schedule.ErrScheduleNotFound.With("schedule", name)
	}

	return a.scheduleController.Status(ctx, found.TopicId, found.Name)
}

// ScheduleMessages is the schedule's newest messages, one ScheduleMessageStatus
// per (message, consumer group that receives it), newest message first.
// Messages older than the target topic's retention window are gone.
// Returns ErrScheduleNotFound if name isn't registered.
func (a *MessageAdmin) ScheduleMessages(ctx context.Context, name string, limit int) ([]*schedule.ScheduleMessageStatus, error) {
	found, err := a.scheduleController.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, schedule.ErrScheduleNotFound.With("schedule", name)
	}

	return a.scheduleController.ListMessages(ctx, found.TopicId, found.Name, limit)
}

// DestroySchedule permanently deletes the schedule. Returns topic.ErrDestroyDisabled
// unless MessageAdminConfig.AllowDestroy is set.
func (a *MessageAdmin) DestroySchedule(ctx context.Context, name string) error {
	if !a.allowDestroy {
		return topic.ErrDestroyDisabled
	}

	return a.scheduleController.Delete(ctx, name)
}
