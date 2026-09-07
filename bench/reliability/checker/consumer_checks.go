package checker

import (
	"context"
	"fmt"
)

// The consumer side: what the group's own tables say happened to it.

// reclaims: deliveries logged 'expired' -- a claim's lease ran out with no
// outcome and another claim took the range over. One row per message in the
// range, so this counts messages a reclaim touched, not reclaim events.
func (c *Checker) reclaims(ctx context.Context, target *target) (measurement, error) {
	reclaimsSql := fmt.Sprintf(`
		-- lab: checker.reclaims
		SELECT
			count(*),
			COALESCE((array_agg(message_id::text ORDER BY message_id))[1:%[2]d], ARRAY[]::text[])
		FROM %[1]s
		WHERE consumer_group_id = $1 AND status = 'expired';
	`, target.deliveryLog(), witnessLimit)
	return c.measure(ctx, witnessMessageId, reclaimsSql, target.groupId)
}

// dead: exception rows the group dead-lettered.
func (c *Checker) dead(ctx context.Context, target *target) (measurement, error) {
	deadSql := fmt.Sprintf(`
		-- lab: checker.dead
		SELECT
			count(*),
			COALESCE((array_agg(message_id::text ORDER BY message_id))[1:%[2]d], ARRAY[]::text[])
		FROM %[1]s
		WHERE consumer_group_id = $1 AND status = 'dead';
	`, target.exceptionQueue(), witnessLimit)
	return c.measure(ctx, witnessMessageId, deadSql, target.groupId)
}
