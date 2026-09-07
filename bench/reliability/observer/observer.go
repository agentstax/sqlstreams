package observer

// observer samples the server once a second for as long as it runs: the
// server's own counters into the sample records, the scenario's consumer
// group position into the backlog records. It writes what it reads and
// judges nothing; the checker takes the differences.

import (
	"context"
	"errors"
	"time"

	"github.com/agentstax/vulkan/bench/reliability/observer/datastore"
	"github.com/agentstax/vulkan/bench/reliability/record"
)

const samplePeriod = time.Second

type Observer struct {
	ds       *datastore.ObserverDatastore
	samples  *record.Writer
	backlogs *record.Writer
	topic    string
	group    string

	target   datastore.Target
	resolved bool
}

func NewObserver(ds *datastore.ObserverDatastore, samples *record.Writer, backlogs *record.Writer, topic string, group string) (*Observer, error) {
	if ds == nil {
		return nil, errors.New("ds must not be nil")
	}
	if samples == nil {
		return nil, errors.New("samples must not be nil")
	}
	if backlogs == nil {
		return nil, errors.New("backlogs must not be nil")
	}
	if topic == "" {
		return nil, errors.New("topic must not be empty")
	}
	if group == "" {
		return nil, errors.New("group must not be empty")
	}
	return &Observer{ds: ds, samples: samples, backlogs: backlogs, topic: topic, group: group}, nil
}

// Run samples until ctx is cancelled and returns ctx.Err(). A read that
// fails skips that second and nothing else: a paused Postgres is a chaos
// phase the observer must outlive, and a run with no samples at all is the
// checker's unknown verdict. A record write failing is a lab failure.
func (o *Observer) Run(ctx context.Context) error {
	ticker := time.NewTicker(samplePeriod)
	defer ticker.Stop()
	for {
		var now time.Time
		select {
		case <-ctx.Done():
			return ctx.Err()
		case now = <-ticker.C:
		}
		if err := o.sample(ctx, now); err != nil {
			return err
		}
	}
}

func (o *Observer) sample(ctx context.Context, now time.Time) error {
	sample, err := o.ds.ReadSample(ctx)
	if err == nil {
		sample.At = now
		if err := o.samples.Write(sample); err != nil {
			return err
		}
	}

	if !o.resolved {
		target, ok, err := o.ds.ResolveTarget(ctx, o.topic, o.group)
		if err != nil || !ok {
			return nil
		}
		o.target, o.resolved = target, true
	}
	backlog, err := o.ds.ReadBacklog(ctx, o.target)
	if err != nil {
		return nil
	}
	backlog.At, backlog.Topic, backlog.Group = now, o.topic, o.group
	return o.backlogs.Write(backlog)
}
