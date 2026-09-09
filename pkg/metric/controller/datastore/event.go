package datastore

import (
	"context"
	"fmt"

	"github.com/agentstax/vulkan/pkg/metric"
	"github.com/agentstax/vulkan/pkg/topic"
)

// EventTimestamps is every distinct (message, attempt) of eventType under
// routingKey on __system.metrics's own message log, with the earliest time
// each was seen.
func (d *MetricDatastore) EventTimestamps(ctx context.Context, routingKey string, eventType metric.EventType) ([]EventTimestampRow, error) {
	var events []EventTimestampRow
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		events, err = d.eventTimestamps(ctx, routingKey, eventType)
		return err
	})
	return events, err
}

func (d *MetricDatastore) eventTimestamps(ctx context.Context, routingKey string, eventType metric.EventType) ([]EventTimestampRow, error) {
	metricTopicId, err := d.resolveMetricsTopicId(ctx)
	if err != nil {
		return nil, err
	}

	sql := fmt.Sprintf(`
		-- vulkan: metric.eventTimestamps
		SELECT
			(payload->>'message_id')::bigint,
			(payload->>'attempt')::int,
			MIN((payload->>'at')::timestamptz)
		FROM %[1]s.%[2]s
		WHERE routing_key = $1 AND payload->>'type' = $2
		GROUP BY (payload->>'message_id')::bigint, (payload->>'attempt')::int;
	`, d.Datastore.Schema, topic.MessageLogTable(metricTopicId))
	rows, err := d.Datastore.Pool.Query(ctx, sql, routingKey, eventType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []EventTimestampRow
	for rows.Next() {
		var data EventTimestampRow
		if err := rows.Scan(&data.MessageId, &data.Attempt, &data.At); err != nil {
			return nil, err
		}
		events = append(events, data)
	}
	return events, rows.Err()
}

// resolveMetricsTopicId is the __system.metrics topic's own id.
func (d *MetricDatastore) resolveMetricsTopicId(ctx context.Context) (int64, error) {
	var id int64
	sql := fmt.Sprintf(`
		-- vulkan: metric.resolveMetricsTopicId
		SELECT id FROM %[1]s.topic_config WHERE name = $1;
	`, d.Datastore.Schema)
	err := d.Datastore.Pool.QueryRow(ctx, sql, metric.MetricTopicName).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}
