// Scenario 06 -- a schedule.
//
// FrameForge's nightly usage report: register the schedule once with the
// message it produces and the topic it produces to, then consume that topic
// like any other.
//
// Concepts held before domain code (12): the 7 from scenario 03, plus
// SchedulerHandle.Register[T] and the returned
// SchedulerInstance's Schedule
// verb, vulkan.MetaFromContext for the scheduled time, and the fact that
// Schedule runs the system manager.
//
// Traps hit:
//   - Nothing produces if no manager is running; Schedule is the manager,
//     so a register-and-exit program registers a schedule that never fires.
//   - The scheduled time is not in the payload (the payload never
//     changes) -- it is on the delivery's meta, reached through ctx.
//   - A one-off "run this in 10 minutes" is not this feature; it is
//     queue-native delayed delivery (future roadmap).
package main

import (
	"context"
	"fmt"
	"os"

	vulkan "github.com/agentstax/vulkan/pkg/vulkan"
	"golang.org/x/sync/errgroup"
)

type UsageReportRequestedV1 struct {
	Scope string `json:"scope"`
}

// increment on breaking changes
func (UsageReportRequestedV1) SchemaVersion() int { return 1 }

func main() {
	if err := run(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := vulkan.LifecycleContext(nil)
	defer stop()

	pool, err := vulkan.NewPostgresPool(ctx, "example_user", "example_password", "localhost", "example_db", nil)
	if err != nil {
		return err
	}
	defer pool.Close()

	client, err := vulkan.NewClient(ctx, pool, nil)
	if err != nil {
		return err
	}
	reports, err := client.Topic[UsageReportRequestedV1]("usage.reports.requested").Register(ctx, nil)
	if err != nil {
		return err
	}

	nightly, err := client.Scheduler("usage.reports.nightly").Register(ctx, reports.Name, "0 2 * * *", &UsageReportRequestedV1{Scope: "all-creators"}, nil)
	if err != nil {
		return err
	}

	builder, err := client.Topic[UsageReportRequestedV1](reports.Name).Consumer("usage-report-builder").Register(ctx, nil)
	if err != nil {
		return err
	}

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error { return nightly.Schedule(groupCtx) })
	group.Go(func() error {
		return builder.Consume(groupCtx, func(ctx context.Context, request *UsageReportRequestedV1) error {
			meta, _ := vulkan.MetaFromContext(ctx)
			fmt.Printf("building %s usage report for %s\n", request.Scope, meta.ScheduledAt.Format("2006-01-02"))
			return nil
		}, nil)
	})
	return group.Wait()
}
