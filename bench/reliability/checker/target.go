package checker

import (
	"context"
	"fmt"

	"github.com/agentstax/vulkan/bench/reliability/ledger"
	"github.com/agentstax/vulkan/pkg/datastore"
	"github.com/agentstax/vulkan/pkg/topic"
)

// vulkanSchema is where the roles' client put vulkan's tables: they run with
// a nil ClientConfig, so the default.
const vulkanSchema = datastore.DefaultSchema

// the loaded ledger's tables, qualified
var (
	produceLedger = ledger.Schema + "." + ledger.ProduceTable
	handlerLedger = ledger.Schema + "." + ledger.HandlerTable
	runPhase      = ledger.Schema + "." + ledger.PhaseTable
)

// target is the topic and consumer group the checks read, resolved from the
// catalog by the names the scenario declares.
type target struct {
	topicId int64
	groupId int64
}

func (c *Checker) resolveTarget(ctx context.Context) (*target, error) {
	resolveSql := fmt.Sprintf(`
		-- lab: checker.resolveTarget
		SELECT t.id, g.id
		FROM %[1]s.topic_config t
		JOIN %[1]s.consumer_group_config g ON g.topic_id = t.id AND g.name = $2
		WHERE t.name = $1;
	`, vulkanSchema)
	resolved := &target{}
	if err := c.pool.QueryRow(ctx, resolveSql, c.declared.Topic, c.declared.Group).Scan(&resolved.topicId, &resolved.groupId); err != nil {
		return nil, fmt.Errorf("consumer group %q on topic %q: %w", c.declared.Group, c.declared.Topic, err)
	}
	return resolved, nil
}

func (t *target) messageLog() string {
	return vulkanSchema + "." + topic.MessageLogTable(t.topicId)
}

func (t *target) deliveryLog() string {
	return vulkanSchema + "." + topic.DeliveryLogTable(t.topicId)
}

func (t *target) exceptionQueue() string {
	return vulkanSchema + "." + topic.ExceptionQueueTable(t.topicId)
}

func (t *target) consumerGroupCursor() string {
	return vulkanSchema + "." + topic.ConsumerGroupCursorTable(t.topicId)
}
