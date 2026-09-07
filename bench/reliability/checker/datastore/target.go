package datastore

import (
	"context"
	"fmt"

	vulkandatastore "github.com/agentstax/vulkan/pkg/datastore"
	"github.com/agentstax/vulkan/pkg/topic"
)

// vulkanSchema is where the roles' client put vulkan's tables: they run with
// a nil ClientConfig, so the default.
const vulkanSchema = vulkandatastore.DefaultSchema

// Target is the topic and consumer group the checks read, resolved from the
// catalog by the names the scenario declares.
type Target struct {
	TopicId int64
	GroupId int64
}

func (d *CheckerDatastore) ResolveTarget(ctx context.Context, topicName string, groupName string) (Target, error) {
	resolveSql := fmt.Sprintf(`
		-- lab: datastore.ResolveTarget
		SELECT t.id, g.id
		FROM %[1]s.topic_config t
		JOIN %[1]s.consumer_group_config g ON g.topic_id = t.id AND g.name = $2
		WHERE t.name = $1;
	`, vulkanSchema)
	var resolved Target
	if err := d.pool.QueryRow(ctx, resolveSql, topicName, groupName).Scan(&resolved.TopicId, &resolved.GroupId); err != nil {
		return Target{}, fmt.Errorf("consumer group %q on topic %q: %w", groupName, topicName, err)
	}
	return resolved, nil
}

func (t Target) messageLog() string {
	return vulkanSchema + "." + topic.MessageLogTable(t.TopicId)
}

func (t Target) deliveryLog() string {
	return vulkanSchema + "." + topic.DeliveryLogTable(t.TopicId)
}

func (t Target) exceptionQueue() string {
	return vulkanSchema + "." + topic.ExceptionQueueTable(t.TopicId)
}

func (t Target) consumerGroupCursor() string {
	return vulkanSchema + "." + topic.ConsumerGroupCursorTable(t.TopicId)
}
