package datastore

import (
	"context"
	"fmt"
	"time"

	sqlstreamsdatastore "github.com/allegedlyreliable/sqlstreams/pkg/datastore"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
)

// sqlstreamsSchema is where the roles' client put sqlstreams's tables: they run with
// a nil ClientConfig, so the default.
const sqlstreamsSchema = sqlstreamsdatastore.DefaultSchema

// Target is one stream and consumer group the checks read: the names the
// scenario declares, which the records carry, and the ids the catalog
// resolved them to, which name the library's tables.
type Target struct {
	Stream       string
	Group        string
	StreamId     int64
	GroupId      int64
	RetentionTTL time.Duration
}

func (d *CheckerDatastore) ResolveTarget(ctx context.Context, streamName string, groupName string) (Target, error) {
	resolveSql := fmt.Sprintf(`
		-- lab: datastore.ResolveTarget
		SELECT t.id, g.id, t.retention_ttl_ns
		FROM %[1]s.stream_config t
		JOIN %[1]s.consumer_group_config g ON g.stream_id = t.id AND g.name = $2
		WHERE t.name = $1;
	`, sqlstreamsSchema)
	resolved := Target{Stream: streamName, Group: groupName}
	if err := d.pool.QueryRow(ctx, resolveSql, streamName, groupName).Scan(&resolved.StreamId, &resolved.GroupId, &resolved.RetentionTTL); err != nil {
		return Target{}, fmt.Errorf("consumer group %q on stream %q: %w", groupName, streamName, err)
	}
	return resolved, nil
}

func (t Target) messageLog() string {
	return sqlstreamsSchema + "." + stream.MessageLogTable(t.StreamId)
}

func (t Target) deliveryLog() string {
	return sqlstreamsSchema + "." + stream.DeliveryLogTable(t.StreamId)
}

func (t Target) exceptionQueue() string {
	return sqlstreamsSchema + "." + stream.ExceptionQueueTable(t.StreamId)
}

func (t Target) consumerGroupCursor() string {
	return sqlstreamsSchema + "." + stream.ConsumerGroupCursorTable(t.StreamId)
}
