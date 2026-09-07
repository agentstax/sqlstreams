package datastore

import (
	"context"
	"fmt"
)

// The consumer side: what the group's own tables say happened to it.

// Reclaims: deliveries logged 'expired' -- a claim's lease ran out with no
// outcome and another claim took the range over. One row per message in the
// range, so this counts messages a reclaim touched, not reclaim events.
func (d *CheckerDatastore) Reclaims(ctx context.Context, target Target) (Measurement, error) {
	reclaimsSql := fmt.Sprintf(`
		-- lab: datastore.Reclaims
		SELECT
			count(*),
			COALESCE((array_agg(message_id::text ORDER BY message_id))[1:%[2]d], ARRAY[]::text[])
		FROM %[1]s
		WHERE consumer_group_id = $1 AND status = 'expired';
	`, target.deliveryLog(), witnessLimit)
	return d.measure(ctx, witnessMessageId, reclaimsSql, target.GroupId)
}

// Dead: exception rows the group dead-lettered.
func (d *CheckerDatastore) Dead(ctx context.Context, target Target) (Measurement, error) {
	deadSql := fmt.Sprintf(`
		-- lab: datastore.Dead
		SELECT
			count(*),
			COALESCE((array_agg(message_id::text ORDER BY message_id))[1:%[2]d], ARRAY[]::text[])
		FROM %[1]s
		WHERE consumer_group_id = $1 AND status = 'dead';
	`, target.exceptionQueue(), witnessLimit)
	return d.measure(ctx, witnessMessageId, deadSql, target.GroupId)
}
