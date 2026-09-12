package sqlstreams

// Package sqlstreams is the one client over a Postgres pool: registration objects
// built once, ambient config held once, and every verb delegated to the
// package that owns it.

import (
	"context"
	"errors"

	"github.com/allegedlyreliable/sqlstreams/pkg/admin"
	"github.com/allegedlyreliable/sqlstreams/pkg/consumer"
	"github.com/allegedlyreliable/sqlstreams/pkg/datastore"
	"github.com/allegedlyreliable/sqlstreams/pkg/producer"
	"github.com/allegedlyreliable/sqlstreams/pkg/scheduler"
	"github.com/allegedlyreliable/sqlstreams/pkg/systemmanager"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Client struct {
	disableManager bool

	ds        *datastore.PostgresDatastore
	admin     *admin.MessageAdmin
	consumer  *consumer.Consumer
	producer  *producer.Producer
	scheduler *scheduler.Scheduler
	manager   *systemmanager.SystemManager
}

// NewClient builds every registration object over pool and pings it once, so a wrong
// address or credential fails here instead of at the first query. The pool
// stays the caller's -- sqlstreams never closes it. cfg may be nil or sparse. Settings are captured at construction, including a copy of Retry;
// later edits to cfg do not reconfigure the client. The supplied logger is shared.
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

	var retry *RetryPolicy
	if cfg.Retry != nil {
		policy := *cfg.Retry
		retry = &policy
	}

	ds, err := datastore.NewPostgresDatastore(ctx, pool, &datastore.PostgresDatastoreConfig{
		Schema: cfg.Schema,
		Logger: cfg.Logger,
		Retry:  retry,
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
		disableManager: cfg.DisableManager,
		ds:             ds,
		admin:          messageAdmin,
		consumer:       messageConsumer,
		producer:       messageProducer,
		scheduler:      messageScheduler,
		manager:        systemManager,
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
