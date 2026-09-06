package main

// Scenario 06 -- a schedule.
//
// FrameForge's nightly usage report: register the schedule once with the
// message it produces and the topic it produces to, then consume that topic
// like any other.

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

	reports := client.Topic[UsageReportRequestedV1]("usage.reports.requested")
	registered, err := reports.Register(ctx, nil)
	if err != nil {
		return err
	}

	nightly := client.Scheduler("usage.reports.nightly")
	scheduler, err := nightly.Register(ctx, registered.Name, "0 2 * * *", &UsageReportRequestedV1{Scope: "all-creators"}, nil)
	if err != nil {
		return err
	}

	builder := reports.Consumer("usage-report-builder")
	consumer, err := builder.Register(ctx, nil)
	if err != nil {
		return err
	}

	routines, routinesCtx := errgroup.WithContext(ctx)
	routines.Go(func() error { return scheduler.Schedule(routinesCtx) })
	routines.Go(func() error { return consumer.Consume(routinesCtx, buildUsageReport, nil) })
	return routines.Wait()
}

func buildUsageReport(ctx context.Context, request *UsageReportRequestedV1) error {
	meta, _ := vulkan.MetaFromContext(ctx)
	fmt.Printf("building %s usage report for %s\n", request.Scope, meta.ScheduledAt.Format("2006-01-02"))
	return nil
}
