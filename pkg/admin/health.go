package admin

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentstax/vulkan/pkg/topic"
)

// TopicHealth is every payload version present in the named topic's log,
// each with its own retire verdict. Returns ErrTopicNotFound if name isn't
// registered; an empty topic has no versions.
func (a *MessageAdmin) TopicHealth(ctx context.Context, name string) ([]*topic.TopicVersionHealth, error) {
	found, err := a.topicController.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, topic.ErrTopicNotFound.With("topic", name)
	}

	snapshots, err := a.metricsController.TopicSchemaVersionSnapshots(ctx, found.Id)
	if err != nil {
		return nil, err
	}

	results := make([]*topic.TopicVersionHealth, 0, len(snapshots))
	for _, snapshot := range snapshots {
		health := &topic.TopicVersionHealth{
			Topic:           found,
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
