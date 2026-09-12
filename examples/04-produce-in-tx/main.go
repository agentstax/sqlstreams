package main

// Scenario 04 -- produce inside the caller's own transaction.
//
// A completed upload and its VideoUploaded message are recorded atomically;
// then the multi-stream form also records billable usage.

import (
	"context"
	"fmt"
	"os"

	"github.com/agentstax/sqlstreams/client"
	"github.com/jackc/pgx/v5/pgxpool"
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

type UsageRecordedV1 struct {
	VideoId      string `json:"video_id"`
	OwnerId      string `json:"owner_id"`
	StorageBytes int64  `json:"storage_bytes"`
}

// increment on breaking changes
func (UsageRecordedV1) SchemaVersion() int { return 1 }

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

	if err := createVideosTable(ctx, pool); err != nil {
		return err
	}

	client, err := sqlstreams.NewClient(ctx, pool, nil)
	if err != nil {
		return err
	}

	uploads := client.Stream[VideoUploadedV1]("videos.uploaded")
	_, err = uploads.Register(ctx, nil)
	if err != nil {
		return err
	}

	usage := client.Stream[UsageRecordedV1]("usage.recorded")
	_, err = usage.Register(ctx, nil)
	if err != nil {
		return err
	}

	uploadsProducer, err := uploads.Producer().Register(ctx, nil)
	if err != nil {
		return err
	}

	usageProducer, err := usage.Producer().Register(ctx, nil)
	if err != nil {
		return err
	}

	// one stream: the message's own transaction carries the business write
	produced, err := uploadsProducer.ProduceFunc(ctx,
		func(ctx context.Context, tx sqlstreams.Tx) (*VideoUploadedV1, error) {
			if _, err := tx.Exec(ctx, `INSERT INTO playground_videos (id, owner_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, "video-42", "creator-7"); err != nil {
				return nil, err
			}
			return &VideoUploadedV1{
				VideoId:         "video-42",
				OwnerId:         "creator-7",
				UploadId:        "upl-123",
				DurationMinutes: 12,
				SourceStatus:    "ready",
			}, nil
		}, nil)
	if err != nil {
		return err
	}
	fmt.Printf("produced id=%d\n", produced.Id)

	// two streams: the caller owns the transaction, each instance produces into it
	if err := client.InTransaction(ctx, func(ctx context.Context, tx sqlstreams.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO playground_videos (id, owner_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, "video-43", "creator-7"); err != nil {
			return err
		}
		video := &VideoUploadedV1{
			VideoId:         "video-43",
			OwnerId:         "creator-7",
			UploadId:        "upl-124",
			DurationMinutes: 48,
			SourceStatus:    "ready",
		}
		if _, err := uploadsProducer.ProduceInTx(ctx, tx, video, nil); err != nil {
			return err
		}
		_, err := usageProducer.ProduceInTx(ctx, tx, &UsageRecordedV1{VideoId: "video-43", OwnerId: "creator-7", StorageBytes: 8_400_000_000}, nil)
		return err
	}); err != nil {
		return err
	}
	fmt.Println("two streams committed together")
	return nil
}

// createVideosTable stands in for a business table an application would
// already have -- without it the scenario cannot run against a fresh database.
func createVideosTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS playground_videos (id text PRIMARY KEY, owner_id text NOT NULL)`)
	return err
}
