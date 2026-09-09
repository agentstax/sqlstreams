package main

// Scenario 10 -- a schedule that produces on a cron expression.
//
// A nightly usage report: register the schedule once with the message it
// produces and the stream it produces to, then consume that stream like any
// other.

import (
	"context"
	"fmt"
	"os"

	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
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
	ctx, stop := sqlstreams.LifecycleContext(nil)
	defer stop()

	pool, err := sqlstreams.NewPostgresPool(ctx, "example_user", "example_password", "localhost", "example_db", nil)
	if err != nil {
		return err
	}
	defer pool.Close()

	client, err := sqlstreams.NewClient(ctx, pool, nil)
	if err != nil {
		return err
	}
	reports, err := client.Stream[UsageReportRequestedV1]("usage.reports.requested").Register(ctx, nil)
	if err != nil {
		return err
	}

	nightly, err := client.Scheduler("usage.reports.nightly").Register(ctx, reports.Name, "0 2 * * *", &UsageReportRequestedV1{Scope: "all-creators"}, nil)
	if err != nil {
		return err
	}

	builder, err := client.Stream[UsageReportRequestedV1](reports.Name).Consumer("usage-report-builder").Register(ctx, nil)
	if err != nil {
		return err
	}

	// Schedule is the process that produces the message; a program that
	// registers the schedule and exits leaves one that never runs
	routines, routinesCtx := errgroup.WithContext(ctx)
	routines.Go(func() error { return nightly.Schedule(routinesCtx) })
	routines.Go(func() error { return builder.Consume(routinesCtx, buildUsageReport, nil) })
	return routines.Wait()
}

func buildUsageReport(ctx context.Context, request *UsageReportRequestedV1) error {
	// the payload is the same every run; the scheduled time is on the delivery's meta
	meta, _ := sqlstreams.MetaFromContext(ctx)
	fmt.Printf("building %s usage report for %s\n", request.Scope, meta.ScheduledAt.Format("2006-01-02"))
	return nil
}
