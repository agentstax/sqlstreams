package datastore

import (
	"context"
	"fmt"
)

// The handler records prove invocation; the committed cursor and exception
// rows prove completion independently of success audit logging.

// Undelivered: messages the lab's handler never succeeded on that the
// library did not dead-letter either. The handler records are the source, so a
// success the library recorded without the handler running counts here.
func (d *CheckerDatastore) CountUndelivered(ctx context.Context, target Target) (Measurement, error) {
	undeliveredSql := fmt.Sprintf(`
		-- lab: datastore.CountUndelivered
		SELECT
			count(*),
			COALESCE((array_agg(m.id::text ORDER BY m.id))[1:%[4]d], ARRAY[]::text[])
		FROM (SELECT id FROM %[1]s UNION SELECT message_id FROM %[5]s WHERE stream = $2 AND kind = 'committed' AND message_id > 0) m
		WHERE NOT EXISTS (
				SELECT 1 FROM %[2]s h
				WHERE h.stream = $2 AND h."group" = $3 AND h.message_id = m.id AND h.outcome = 'success')
			AND NOT EXISTS (
				SELECT 1 FROM %[3]s e
				WHERE e.consumer_group_id = $1 AND e.message_id = m.id AND e.status = 'dead');
	`, target.messageLog(), handlerRecord, target.exceptionQueue(), exampleLimit, produceRecord)
	return d.measure(ctx, exampleMessageId, undeliveredSql, target.GroupId, target.Stream, target.Group)
}

// Duplicates: messages the group's handler succeeded on more than once, by
// the records' own count -- the redelivery the lease contract allows.
func (d *CheckerDatastore) CountDuplicates(ctx context.Context, target Target) (Measurement, error) {
	duplicatesSql := fmt.Sprintf(`
		-- lab: datastore.CountDuplicates
		WITH repeated AS (
			SELECT message_id
			FROM %[1]s
			WHERE stream = $1 AND "group" = $2 AND outcome = 'success'
			GROUP BY message_id
			HAVING count(*) > 1
		)
		SELECT
			count(*),
			COALESCE((array_agg(message_id::text ORDER BY message_id))[1:%[2]d], ARRAY[]::text[])
		FROM repeated;
	`, handlerRecord, exampleLimit)
	return d.measure(ctx, exampleMessageId, duplicatesSql, target.Stream, target.Group)
}

// Unbucketed: messages not completed by the durable cursor or dead-lettered.
// Undelivered separately checks that the handler actually ran.
func (d *CheckerDatastore) CountUnbucketed(ctx context.Context, target Target) (Measurement, error) {
	unbucketedSql := fmt.Sprintf(`
		-- lab: datastore.CountUnbucketed
		WITH buckets AS (
			SELECT
				m.id,
				(m.id <= (SELECT COALESCE(max(committed), 0) FROM %[2]s WHERE consumer_group_id = $1)
				 AND NOT EXISTS (
					SELECT 1 FROM %[3]s e
					WHERE e.consumer_group_id = $1 AND e.message_id = m.id AND e.status <> 'done')) AS success,
				EXISTS (
					SELECT 1 FROM %[3]s e
					WHERE e.consumer_group_id = $1 AND e.message_id = m.id AND e.status = 'dead') AS dead
			FROM (SELECT id FROM %[1]s UNION SELECT message_id FROM %[5]s WHERE stream = $2 AND kind = 'committed' AND message_id > 0) m
		)
		SELECT
			count(*),
			COALESCE((array_agg(id::text ORDER BY id))[1:%[4]d], ARRAY[]::text[])
		FROM buckets
		WHERE (success AND dead) OR (NOT success AND NOT dead);
	`, target.messageLog(), target.consumerGroupCursor(), target.exceptionQueue(), exampleLimit, produceRecord)
	return d.measure(ctx, exampleMessageId, unbucketedSql, target.GroupId, target.Stream)
}
