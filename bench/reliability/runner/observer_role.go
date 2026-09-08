package runner

import (
	"context"

	"github.com/agentstax/vulkan/bench/reliability/observer"
	"github.com/agentstax/vulkan/bench/reliability/observer/datastore"
	"github.com/agentstax/vulkan/bench/reliability/record"
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
	for _, declared := range r.declared.Topics {
		for _, group := range declared.Groups {
			groups = append(groups, observer.GroupName{Topic: declared.Name, Group: group.Name})
		}
	}
	sampler, err := observer.NewObserver(ds, sampleRecords, backlogRecords, groups)
	if err != nil {
		return err
	}
	return ignoreCancellation(sampler.Run(ctx))
}
