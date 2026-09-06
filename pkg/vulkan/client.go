package vulkan

// Package vulkan is the one client over a Postgres pool: registration objects
// built once, ambient config held once, and every verb delegated to the
// package that owns it.

import (
	"context"
	"errors"

	"github.com/agentstax/vulkan/pkg/admin"
	"github.com/agentstax/vulkan/pkg/common/logging"
	"github.com/agentstax/vulkan/pkg/consumer"
	"github.com/agentstax/vulkan/pkg/datastore"
	"github.com/agentstax/vulkan/pkg/producer"
	"github.com/agentstax/vulkan/pkg/scheduler"
	"github.com/agentstax/vulkan/pkg/systemmanager"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Client struct {
	Config *ClientConfig
	Logger logging.Logger

	ds        *datastore.PostgresDatastore
	admin     *admin.MessageAdmin
	consumer  *consumer.Consumer
	producer  *producer.Producer
	scheduler *scheduler.Scheduler
	manager   *systemmanager.SystemManager
}

// NewClient builds every registration object over pool and pings it once, so a wrong
// address or credential fails here instead of at the first query. The pool
// stays the caller's -- vulkan never closes it. cfg may be nil or a sparse
// struct.
func NewClient(ctx context.Context, pool *pgxpool.Pool, cfg *ClientConfig) (*Client, error) {
	if pool == nil {
		return nil, errors.New("pool must not be nil")
	}
	if cfg == nil {
		cfg = &ClientConfig{}
	}
	cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	ds, err := datastore.NewPostgresDatastore(ctx, pool, &datastore.PostgresDatastoreConfig{
		Schema: cfg.Schema,
		Logger: cfg.Logger,
		Retry:  cfg.Retry,
	})
	if err != nil {
		return nil, err
	}

	messageAdmin, err := admin.NewMessageAdmin(ds, &admin.MessageAdminConfig{AllowDestroy: cfg.AllowDestroy})
	if err != nil {
		return nil, err
	}

	messageConsumer, err := consumer.NewConsumer(ds)
	if err != nil {
		return nil, err
	}
	messageProducer, err := producer.NewProducer(ds)
	if err != nil {
		return nil, err
	}
	messageScheduler, err := scheduler.NewScheduler(ds)
	if err != nil {
		return nil, err
	}

	systemManager, err := systemmanager.NewSystemManager(ds, nil)
	if err != nil {
		return nil, err
	}

	return &Client{
		Config:    cfg,
		Logger:    ds.Logger,
		ds:        ds,
		admin:     messageAdmin,
		consumer:  messageConsumer,
		producer:  messageProducer,
		scheduler: messageScheduler,
		manager:   systemManager,
	}, nil
}

// InTransaction opens one transaction, runs transactionFunc against it, and
// commits -- the way to publish to multiple targets atomically via ProduceInTx.
//
// It does not retry -- a transient blip or an ambiguous commit failure
// surfaces to you as-is. Wrap your own retry loop around it if you want one;
// only you know what's safe to rerun in your closure. Rerunning the whole
// closure is dedup-safe ONLY under caller-supplied IdempotencyKeys -- unset
// keys mint fresh per call, so a rerun double-publishes.
func (c *Client) InTransaction(ctx context.Context, transactionFunc TransactionFunc) error {
	return datastore.InTransaction(ctx, c.ds, transactionFunc)
}
