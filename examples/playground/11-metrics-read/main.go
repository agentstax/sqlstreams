package main

// Scenario 11 -- reading what the system measures about itself.
//
// The transcoder from scenario 02 consumes the uploads from scenario 01. This
// program reads that group's metrics two ways and exits: the live picture
// computed from the tables right now, and the last value the manager's
// collector stored -- the number a dashboard or alert reads.
//
// Run first: 01, then 02

import (
	"fmt"
	"os"
	"time"

	vulkan "github.com/agentstax/vulkan/pkg/vulkan"
)

type VideoUploadedV1 struct {
	VideoId         string `json:"video_id"`
	OwnerId         string `json:"owner_id"`
	UploadId        string `json:"upload_id"`
	DurationMinutes int    `json:"duration_minutes"`
	SourceStatus    string `json:"source_status"`
	ReleaseAtUnix   int64  `json:"release_at_unix"`
}

// increment on breaking changes
func (VideoUploadedV1) SchemaVersion() int { return 1 }

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

	uploads := client.Topic[VideoUploadedV1]("videos.uploaded")
	transcoder := uploads.Consumer("transcoder")

	// Snapshot -> computed from the group's tables at this instant
	snapshot, err := transcoder.Metrics().Snapshot(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("live: head %d, committed %d, backlog %d, dead %d\n",
		snapshot.Cursor.Head, snapshot.Cursor.Committed, snapshot.Cursor.Backlog, snapshot.Exceptions.Dead)

	// Latest -> the collector's last stored value; nil until a manager
	// (any running consumer or scheduler) has completed a collector poll
	collected, err := transcoder.Metrics().CursorBacklog().Latest(ctx)
	if err != nil {
		return err
	}
	if collected == nil {
		fmt.Println("collected: nothing yet")
		return nil
	}
	fmt.Printf("collected: backlog %g, %s ago\n", collected.Value, time.Since(collected.At).Round(time.Second))
	return nil
}
