package datastore

import (
	"context"
	"fmt"
)

// The produce side: the records' committed and unknown keys against the
// message_log rows the library kept.

// Lost: committed produces whose message row is missing.
func (d *CheckerDatastore) CountLost(ctx context.Context, target Target) (Measurement, error) {
	lostSql := fmt.Sprintf(`
		-- lab: datastore.CountLost
		SELECT
			count(*),
			COALESCE((array_agg(p.key ORDER BY p.message_id))[1:%[3]d], ARRAY[]::text[])
		FROM %[1]s p
		WHERE p.stream = $1 AND p.kind = 'committed'
			AND NOT EXISTS (SELECT 1 FROM %[2]s m WHERE m.id = p.message_id);
	`, produceRecord, target.messageLog(), exampleLimit)
	return d.measure(ctx, exampleKey, lostSql, target.Stream)
}

// Unexpected: message rows whose key the records never committed and never
// lost track of -- a rejected produce that landed, or a row nobody attempted.
// The key is rebuilt from the payload as common.Order.Key builds it.
func (d *CheckerDatastore) CountUnexpected(ctx context.Context, target Target) (Measurement, error) {
	unexpectedSql := fmt.Sprintf(`
		-- lab: datastore.CountUnexpected
		WITH messages AS (
			SELECT id, (payload->>'producer') || '-' || (payload->>'sequence') AS key
			FROM %[2]s
		)
		SELECT
			count(*),
			COALESCE((array_agg(m.id::text ORDER BY m.id))[1:%[3]d], ARRAY[]::text[])
		FROM messages m
		WHERE NOT EXISTS (
			SELECT 1 FROM %[1]s p
			WHERE p.stream = $1 AND p.key = m.key AND p.kind IN ('committed', 'unknown')
		);
	`, produceRecord, target.messageLog(), exampleLimit)
	return d.measure(ctx, exampleMessageId, unexpectedSql, target.Stream)
}

// Recovered: produces whose reply was lost but whose row is there.
func (d *CheckerDatastore) CountRecovered(ctx context.Context, target Target) (Measurement, error) {
	recoveredSql := fmt.Sprintf(`
		-- lab: datastore.CountRecovered
		WITH messages AS (
			SELECT (payload->>'producer') || '-' || (payload->>'sequence') AS key
			FROM %[2]s
		)
		SELECT
			count(*),
			COALESCE((array_agg(p.key ORDER BY p.sequence))[1:%[3]d], ARRAY[]::text[])
		FROM %[1]s p
		WHERE p.stream = $1 AND p.kind = 'unknown'
			AND EXISTS (SELECT 1 FROM messages m WHERE m.key = p.key);
	`, produceRecord, target.messageLog(), exampleLimit)
	return d.measure(ctx, exampleKey, recoveredSql, target.Stream)
}
