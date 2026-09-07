package datastore

import (
	"context"
	"fmt"
)

// The produce side: the records' committed and unknown keys against the
// message_log rows the library kept.

// Lost: committed produces whose message row is missing.
func (d *CheckerDatastore) Lost(ctx context.Context, target Target) (Measurement, error) {
	lostSql := fmt.Sprintf(`
		-- lab: datastore.Lost
		SELECT
			count(*),
			COALESCE((array_agg(p.key ORDER BY p.message_id))[1:%[3]d], ARRAY[]::text[])
		FROM %[1]s p
		WHERE p.kind = 'committed'
			AND NOT EXISTS (SELECT 1 FROM %[2]s m WHERE m.id = p.message_id);
	`, produceLedger, target.messageLog(), witnessLimit)
	return d.measure(ctx, witnessKey, lostSql)
}

// Unexpected: message rows whose key the records never committed and never
// lost track of -- a rejected produce that landed, or a row nobody attempted.
// The key is rebuilt from the payload as common.Order.Key builds it.
func (d *CheckerDatastore) Unexpected(ctx context.Context, target Target) (Measurement, error) {
	unexpectedSql := fmt.Sprintf(`
		-- lab: datastore.Unexpected
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
	return d.measure(ctx, witnessMessageId, unexpectedSql)
}

// Recovered: produces whose reply was lost but whose row is there.
func (d *CheckerDatastore) Recovered(ctx context.Context, target Target) (Measurement, error) {
	recoveredSql := fmt.Sprintf(`
		-- lab: datastore.Recovered
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
	return d.measure(ctx, witnessKey, recoveredSql)
}
