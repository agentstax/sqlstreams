package admin

import (
	"context"
	"fmt"
	"strings"

	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
)

// StreamHealth is every payload version present in the named stream's log,
// each with its own retire verdict. Returns ErrStreamNotFound if name isn't
// registered; an empty stream has no versions.
func (a *MessageAdmin) StreamHealth(ctx context.Context, name string) ([]*stream.StreamVersionHealth, error) {
	found, err := a.streamController.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, stream.ErrStreamNotFound.With("stream", name)
	}

	snapshots, err := a.metricController.StreamSchemaVersionSnapshots(ctx, found.Id)
	if err != nil {
		return nil, err
	}

	results := make([]*stream.StreamVersionHealth, 0, len(snapshots))
	for _, snapshot := range snapshots {
		health := &stream.StreamVersionHealth{
			Stream:          found,
			Version:         snapshot.Version,
			Messages:        snapshot.Messages,
			CompactionHeads: snapshot.CompactionHeads,
			Groups:          snapshot.Groups,
		}

		if snapshot.CompactionHeads > 0 {
			health.Reason = fmt.Sprintf("compaction heads remain: %d keys still resolve to this version", snapshot.CompactionHeads)
		} else {
			var lagging []string
			for _, group := range snapshot.Groups {
				if group.Unconsumed > 0 || group.UnresolvedExceptions > 0 {
					lagging = append(lagging, group.ConsumerGroup)
				}
			}

			if len(lagging) > 0 {
				health.Reason = fmt.Sprintf("not drained: %s", strings.Join(lagging, ", "))
			} else {
				health.Safe = true
				health.Reason = "safe: no compaction head points at this version and every group has read past it"
			}
		}

		results = append(results, health)
	}
	return results, nil
}
