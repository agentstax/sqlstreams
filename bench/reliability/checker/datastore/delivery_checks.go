package datastore

import (
	"context"
	"fmt"
)

// The delivery side: every message row sorted into its bucket. Two sources
// say a message reached its end: the handler records, written by the lab's own
// handler, and the library's tables -- a 'success' delivery_log row (mode
// 'all' writes one) or a 'dead' exception row.

// Undelivered: messages the lab's handler never succeeded on that the
// library did not dead-letter either. The handler records are the source, so a
// success the library recorded without the handler running counts here.
func (d *CheckerDatastore) CountUndelivered(ctx context.Context, target Target) (Measurement, error) {
	undeliveredSql := fmt.Sprintf(`
		-- lab: datastore.CountUndelivered
		SELECT
			count(*),
			COALESCE((array_agg(m.id::text ORDER BY m.id))[1:%[4]d], ARRAY[]::text[])
		FROM %[1]s m
		WHERE NOT EXISTS (
				SELECT 1 FROM %[2]s h
				WHERE h.topic = $2 AND h."group" = $3 AND h.message_id = m.id AND h.outcome = 'success')
			AND NOT EXISTS (
				SELECT 1 FROM %[3]s e
				WHERE e.consumer_group_id = $1 AND e.message_id = m.id AND e.status = 'dead');
	`, target.messageLog(), handlerRecord, target.exceptionQueue(), exampleLimit)
	return d.measure(ctx, exampleMessageId, undeliveredSql, target.GroupId, target.Topic, target.Group)
}

// Duplicates: messages the group's handler succeeded on more than once, by
// the records' own count -- the redelivery the lease contract allows.
func (d *CheckerDatastore) CountDuplicates(ctx context.Context, target Target) (Measurement, error) {
	duplicatesSql := fmt.Sprintf(`
		-- lab: datastore.CountDuplicates
		WITH repeated AS (
			SELECT message_id
			FROM %[1]s
			WHERE topic = $1 AND "group" = $2 AND outcome = 'success'
			GROUP BY message_id
			HAVING count(*) > 1
		)
		SELECT
			count(*),
			COALESCE((array_agg(message_id::text ORDER BY message_id))[1:%[2]d], ARRAY[]::text[])
		FROM repeated;
	`, handlerRecord, exampleLimit)
	return d.measure(ctx, exampleMessageId, duplicatesSql, target.Topic, target.Group)
}

// Unbucketed: by the library's own tables, messages in no bucket or in both,
// so the sum produced = success + dead holds exactly when this is zero.
func (d *CheckerDatastore) CountUnbucketed(ctx context.Context, target Target) (Measurement, error) {
	unbucketedSql := fmt.Sprintf(`
		-- lab: datastore.CountUnbucketed
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
	`, target.messageLog(), target.deliveryLog(), target.exceptionQueue(), exampleLimit)
	return d.measure(ctx, exampleMessageId, unbucketedSql, target.GroupId)
}
