package checker

import (
	"context"
	"fmt"
)

// The produce side: the records' committed and unknown keys against the
// message_log rows the library kept.

// lost: committed produces whose message row is missing.
func (c *Checker) lost(ctx context.Context, target *target) (measurement, error) {
	lostSql := fmt.Sprintf(`
		-- lab: checker.lost
		SELECT
			count(*),
			COALESCE((array_agg(p.key ORDER BY p.message_id))[1:%[3]d], ARRAY[]::text[])
		FROM %[1]s p
		WHERE p.kind = 'committed'
			AND NOT EXISTS (SELECT 1 FROM %[2]s m WHERE m.id = p.message_id);
	`, produceLedger, target.messageLog(), witnessLimit)
	return c.measure(ctx, lostSql)
}

// unexpected: message rows whose key the records never committed and never
// lost track of -- a rejected produce that landed, or a row nobody attempted.
// The key is rebuilt from the payload as common.Order.Key builds it.
func (c *Checker) unexpected(ctx context.Context, target *target) (measurement, error) {
	unexpectedSql := fmt.Sprintf(`
		-- lab: checker.unexpected
		WITH messages AS (
			SELECT id, (payload->>'producer') || '-' || (payload->>'seq') AS key
			FROM %[2]s
		)
		SELECT
			count(*),
			COALESCE((array_agg(m.id::text ORDER BY m.id))[1:%[3]d], ARRAY[]::text[])
		FROM messages m
		WHERE NOT EXISTS (
			SELECT 1 FROM %[1]s p
			WHERE p.key = m.key AND p.kind IN ('committed', 'unknown')
		);
	`, produceLedger, target.messageLog(), witnessLimit)
	return c.measure(ctx, unexpectedSql)
}

// recovered: produces whose reply was lost but whose row is there.
func (c *Checker) recovered(ctx context.Context, target *target) (measurement, error) {
	recoveredSql := fmt.Sprintf(`
		-- lab: checker.recovered
		WITH messages AS (
			SELECT (payload->>'producer') || '-' || (payload->>'seq') AS key
			FROM %[2]s
		)
		SELECT
			count(*),
			COALESCE((array_agg(p.key ORDER BY p.seq))[1:%[3]d], ARRAY[]::text[])
		FROM %[1]s p
		WHERE p.kind = 'unknown'
			AND EXISTS (SELECT 1 FROM messages m WHERE m.key = p.key);
	`, produceLedger, target.messageLog(), witnessLimit)
	return c.measure(ctx, recoveredSql)
}
