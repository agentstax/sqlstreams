package runner

import (
	"context"

	"github.com/agentstax/sqlstreams/.bench/observer"
	"github.com/agentstax/sqlstreams/.bench/observer/datastore"
	"github.com/agentstax/sqlstreams/.bench/record"
)

// RunObserver samples the server and the scenario's consumer group once a
// second until ctx is cancelled. It registers nothing: the group's position
// is sampled from the moment the consumer role has registered it.
func (r *Runner) RunObserver(ctx context.Context) error {
	ds, err := datastore.NewObserverDatastore(r.connection.Pool)
	if err != nil {
		return err
	}
	sampleRecords, err := r.openWriter(record.FileKindSample)
	if err != nil {
		return err
	}
	defer sampleRecords.Close()
	backlogRecords, err := r.openWriter(record.FileKindBacklog)
	if err != nil {
		return err
	}
	defer backlogRecords.Close()
	groups := []observer.GroupName{}
	for _, declared := range r.declared.Streams {
		for _, group := range declared.Groups {
			groups = append(groups, observer.GroupName{Stream: declared.Name, Group: group.Name})
		}
	}
	sampler, err := observer.NewObserver(ds, sampleRecords, backlogRecords, groups)
	if err != nil {
		return err
	}
	return ignoreCancellation(sampler.Run(ctx))
}
