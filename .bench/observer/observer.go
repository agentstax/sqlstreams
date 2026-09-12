package observer

// observer samples the server once a second for as long as it runs: the
// server's own counters into the sample records, its statement counters per
// owner into the statement records, the scenario's consumer group position
// into the backlog records. It writes what it reads and
// judges nothing; the checker takes the differences.

import (
	"context"
	"errors"
	"time"

	"github.com/allegedlyreliable/sqlstreams/.bench/observer/datastore"
	"github.com/allegedlyreliable/sqlstreams/.bench/record"
)

const samplePeriod = time.Second

// backlogReadsPerSample caps the consumer groups whose position is read in
// one sample; a scenario with more groups is read round-robin, each group
// every len(groups)/backlogReadsPerSample seconds, so the observer's own
// reads stay a fixed share of the server.
const backlogReadsPerSample = 50

type Observer struct {
	ds         *datastore.ObserverDatastore
	samples    *record.Writer
	statements *record.Writer
	backlogs   *record.Writer
	groups     []GroupName

	// resolved fills per group as the consumer role registers it
	resolved map[GroupName]datastore.Target
	// next is the group the next sample's backlog reads start from
	next int
}

// GroupName is one consumer group to sample, by the names the scenario
// declares.
type GroupName struct {
	Stream string
	Group  string
}

func NewObserver(ds *datastore.ObserverDatastore, samples *record.Writer, statements *record.Writer, backlogs *record.Writer, groups []GroupName) (*Observer, error) {
	if ds == nil {
		return nil, errors.New("ds must not be nil")
	}
	if samples == nil {
		return nil, errors.New("samples must not be nil")
	}
	if statements == nil {
		return nil, errors.New("statements must not be nil")
	}
	if backlogs == nil {
		return nil, errors.New("backlogs must not be nil")
	}
	if len(groups) == 0 {
		return nil, errors.New("groups must not be empty")
	}
	return &Observer{ds: ds, samples: samples, statements: statements, backlogs: backlogs, groups: groups, resolved: map[GroupName]datastore.Target{}}, nil
}

// Run samples until ctx is cancelled and returns ctx.Err(). A read that
// fails skips that second and nothing else: a paused Postgres is a chaos
// phase the observer must outlive, a run with no samples at all is the
// checker's unknown verdict, and a server without pg_stat_statements leaves
// the statement records empty. A record write failing is a lab failure.
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

	statements, err := o.ds.ReadStatements(ctx)
	if err == nil {
		for _, statement := range statements {
			statement.At = now
			if err := o.statements.Write(statement); err != nil {
				return err
			}
		}
	}

	for range min(len(o.groups), backlogReadsPerSample) {
		group := o.groups[o.next]
		o.next = (o.next + 1) % len(o.groups)
		target, ok := o.resolved[group]
		if !ok {
			target, ok, err = o.ds.ResolveTarget(ctx, group.Stream, group.Group)
			if err != nil || !ok {
				continue
			}
			o.resolved[group] = target
		}
		backlog, err := o.ds.ReadBacklog(ctx, target)
		if err != nil {
			continue
		}
		backlog.At, backlog.Stream, backlog.Group = now, group.Stream, group.Group
		if err := o.backlogs.Write(backlog); err != nil {
			return err
		}
	}
	return nil
}
