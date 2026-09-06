package main

// Scenario 12 -- FrameForge consumes __system.alerts as its pager feed.
//
// The built-in checks (partition_count, compaction_read_cost,
// worker_liveness) run as schedules under the manager and produce Alert
// messages; a consumer group on the alert topic is the push integration the
// platform's PagerDuty hook would use. The checks are re-declared here at
// every-minute so a run has any chance of seeing one.

import (
	"context"
	"fmt"
	"os"

	vulkan "github.com/agentstax/vulkan/pkg/vulkan"
)

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

	// newest declaration wins: every minute instead of the @hourly default
	if err := client.System().Register(ctx, &vulkan.SystemConfig{
		PartitionCountAlert:     &vulkan.PartitionCountAlertConfig{ScheduleExpression: "* * * * *"},
		CompactionReadCostAlert: &vulkan.CompactionReadCostAlertConfig{ScheduleExpression: "* * * * *"},
		WorkerLivenessAlert:     &vulkan.WorkerLivenessAlertConfig{ScheduleExpression: "* * * * *"},
	}); err != nil {
		return err
	}

	// the pull side: what is active or resolved right now
	current, err := client.System().Alerts().Latest(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("%d current alerts at startup\n", len(current))

	alerts := client.Topic[vulkan.Alert](vulkan.AlertTopicName)
	pager := alerts.Consumer("frameforge-pager")
	consumer, err := pager.Register(ctx, nil)
	if err != nil {
		return err
	}

	return consumer.Consume(ctx, handleAlert, nil)
}

func handleAlert(ctx context.Context, foundAlert *vulkan.Alert) error {
	fmt.Printf("[%s] %s %s: %s -- %s\n",
		foundAlert.Severity, foundAlert.Status, foundAlert.Name, foundAlert.Message, foundAlert.Hint)
	return nil
}
