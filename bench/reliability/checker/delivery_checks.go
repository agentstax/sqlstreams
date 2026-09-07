package checker

import (
	"context"
	"fmt"
)

// The delivery side: every message row sorted into its bucket. Two witnesses
// say a message reached its end: the handler ledger, written by the lab's own
// handler, and the library's tables -- a 'success' delivery_log row (mode
// 'all' writes one) or a 'dead' exception row.

// undelivered: messages the lab's handler never succeeded on that the
// library did not dead-letter either. The handler ledger is the witness, so a
// success the library recorded without the handler running counts here.
func (c *Checker) undelivered(ctx context.Context, target *target) (measurement, error) {
	undeliveredSql := fmt.Sprintf(`
		-- lab: checker.undelivered
		SELECT
			count(*),
			COALESCE((array_agg(m.id::text ORDER BY m.id))[1:%[4]d], ARRAY[]::text[])
		FROM %[1]s m
		WHERE NOT EXISTS (
				SELECT 1 FROM %[2]s h
				WHERE h.message_id = m.id AND h.outcome = 'success')
			AND NOT EXISTS (
				SELECT 1 FROM %[3]s e
				WHERE e.consumer_group_id = $1 AND e.message_id = m.id AND e.status = 'dead');
	`, target.messageLog(), handlerLedger, target.exceptionQueue(), witnessLimit)
	return c.measure(ctx, undeliveredSql, target.groupId)
}

// duplicates: messages the handler succeeded on more than once, by the
// ledger's own count -- the redelivery the lease contract allows.
func (c *Checker) duplicates(ctx context.Context) (measurement, error) {
	duplicatesSql := fmt.Sprintf(`
		-- lab: checker.duplicates
		WITH repeated AS (
			SELECT message_id
			FROM %[1]s
			WHERE outcome = 'success'
			GROUP BY message_id
			HAVING count(*) > 1
		)
		SELECT
			count(*),
			COALESCE((array_agg(message_id::text ORDER BY message_id))[1:%[2]d], ARRAY[]::text[])
		FROM repeated;
	`, handlerLedger, witnessLimit)
	return c.measure(ctx, duplicatesSql)
}

// unbucketed: by the library's own tables, messages in no bucket or in both,
// so the sum produced = success + dead holds exactly when this is zero.
func (c *Checker) unbucketed(ctx context.Context, target *target) (measurement, error) {
	unbucketedSql := fmt.Sprintf(`
		-- lab: checker.unbucketed
		WITH buckets AS (
			SELECT
				m.id,
				EXISTS (
					SELECT 1 FROM %[2]s d
					WHERE d.consumer_group_id = $1 AND d.message_id = m.id AND d.status = 'success') AS success,
				EXISTS (
					SELECT 1 FROM %[3]s e
					WHERE e.consumer_group_id = $1 AND e.message_id = m.id AND e.status = 'dead') AS dead
			FROM %[1]s m
		)
		SELECT
			count(*),
			COALESCE((array_agg(id::text ORDER BY id))[1:%[4]d], ARRAY[]::text[])
		FROM buckets
		WHERE (success AND dead) OR (NOT success AND NOT dead);
	`, target.messageLog(), target.deliveryLog(), target.exceptionQueue(), witnessLimit)
	return c.measure(ctx, unbucketedSql, target.groupId)
}
