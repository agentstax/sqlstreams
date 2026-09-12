package runner

import (
	"context"
	"fmt"
	"os"

	"github.com/allegedlyreliable/sqlstreams/.bench/observer"
	"github.com/allegedlyreliable/sqlstreams/.bench/observer/datastore"
	"github.com/allegedlyreliable/sqlstreams/.bench/record"
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
	statementRecords, err := r.openWriter(record.FileKindStatement)
	if err != nil {
		return err
	}
	defer statementRecords.Close()
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
	sampler, err := observer.NewObserver(ds, sampleRecords, statementRecords, backlogRecords, groups)
	if err != nil {
		return err
	}
	// a server started without the module cannot create the view; the
	// statement records stay empty and the checker reports them unavailable
	if err := ds.CreateStatementsExtension(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "statement attribution unavailable:", err)
	}
	return ignoreCancellation(sampler.Run(ctx))
}
