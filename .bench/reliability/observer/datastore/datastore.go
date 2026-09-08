package datastore

// datastore is every query the observer runs, all reads: the server's own
// statistics views and one consumer group's position in vulkan's tables.

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agentstax/vulkan/.bench/reliability/record"
	vulkandatastore "github.com/agentstax/vulkan/pkg/datastore"
	"github.com/agentstax/vulkan/pkg/topic"
)

// vulkanSchema is where the roles' client put vulkan's tables: they run with
// a nil ClientConfig, so the default.
const vulkanSchema = vulkandatastore.DefaultSchema

type ObserverDatastore struct {
	pool *pgxpool.Pool
}

func NewObserverDatastore(pool *pgxpool.Pool) (*ObserverDatastore, error) {
	if pool == nil {
		return nil, errors.New("pool must not be nil")
	}
	return &ObserverDatastore{pool: pool}, nil
}

// Target is the topic and consumer group whose position is sampled,
// resolved from the catalog by the names the scenario declares.
type Target struct {
	TopicId int64
	GroupId int64
}

// ReadSample reads the server's cumulative counters: WAL, checkpoints, and
// the current database's transactions, deadlocks, and buffer traffic. At is
// left for the caller to set.
func (d *ObserverDatastore) ReadSample(ctx context.Context) (record.SampleRecord, error) {
	sampleSql := `
		-- lab: datastore.ReadSample
		SELECT
			w.wal_records,
			w.wal_fpi,
			w.wal_bytes,
			c.num_done,
			d.xact_commit,
			d.deadlocks,
			d.blks_hit,
			d.blks_read
		FROM pg_stat_wal w, pg_stat_checkpointer c, pg_stat_database d
		WHERE d.datname = current_database();
	`
	var sample record.SampleRecord
	err := d.pool.QueryRow(ctx, sampleSql).Scan(
		&sample.WalRecords, &sample.WalFpi, &sample.WalBytes, &sample.Checkpoints,
		&sample.XactCommit, &sample.Deadlocks, &sample.BlocksHit, &sample.BlocksRead)
	return sample, err
}

// ResolveTarget is comma-ok: false until the consumer role has registered
// the topic and group.
func (d *ObserverDatastore) ResolveTarget(ctx context.Context, topicName string, groupName string) (Target, bool, error) {
	resolveSql := fmt.Sprintf(`
		-- lab: datastore.ResolveTarget
		SELECT t.id, g.id
		FROM %[1]s.topic_config t
		JOIN %[1]s.consumer_group_config g ON g.topic_id = t.id AND g.name = $2
		WHERE t.name = $1;
	`, vulkanSchema)
	var resolved Target
	err := d.pool.QueryRow(ctx, resolveSql, topicName, groupName).Scan(&resolved.TopicId, &resolved.GroupId)
	if errors.Is(err, pgx.ErrNoRows) {
		return Target{}, false, nil
	}
	if err != nil {
		return Target{}, false, err
	}
	return resolved, true, nil
}

// ReadBacklog reads the topic's highest message id and the group's committed
// cursor; Topic, Group, and At are left for the caller to set.
func (d *ObserverDatastore) ReadBacklog(ctx context.Context, target Target) (record.BacklogRecord, error) {
	backlogSql := fmt.Sprintf(`
		-- lab: datastore.ReadBacklog
		SELECT
			(SELECT COALESCE(max(id), 0) FROM %[1]s),
			(SELECT COALESCE(max(committed), 0) FROM %[2]s WHERE consumer_group_id = $1);
	`, vulkanSchema+"."+topic.MessageLogTable(target.TopicId), vulkanSchema+"."+topic.ConsumerGroupCursorTable(target.TopicId))
	var backlog record.BacklogRecord
	err := d.pool.QueryRow(ctx, backlogSql, target.GroupId).Scan(&backlog.HighestMessage, &backlog.Committed)
	return backlog, err
}
