package datastore

import (
	"context"
	"fmt"

	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/datastore"
)

// migration_log stores exactly one owner column per row, so each read matches
// its own column and pins the other two to NULL.

// SystemSchemaState is the system's version facts, read on the pool: current
// from the latest-by-id success row, minimum compatible from the strictest
// step at or below it.
func (d *MigrateDatastore) SystemSchemaState(ctx context.Context, systemId int64) (*SchemaStateRow, error) {
	var state *SchemaStateRow
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		state, err = d.systemSchemaState(ctx, d.Datastore.Pool, systemId)
		return err
	})
	return state, err
}

// SystemSchemaStateInTx reads the system's version facts through tx.
func (d *MigrateDatastore) SystemSchemaStateInTx(ctx context.Context, tx datastore.Tx, systemId int64) (*SchemaStateRow, error) {
	return d.systemSchemaState(ctx, tx, systemId)
}

func (d *MigrateDatastore) systemSchemaState(ctx context.Context, q datastore.Querier, systemId int64) (*SchemaStateRow, error) {
	sql := fmt.Sprintf(`
		-- vulkan: migrate.systemSchemaState
		WITH successes AS (
			SELECT id, version, min_compatible_version
			FROM %[1]s.migration_log
			WHERE system_id = $1
				AND topic_id IS NULL
				AND consumer_group_id IS NULL
				AND status = 'success'
		),
		current AS (
			-- latest-by-id, not MAX -- a downgrade records a lower version
			SELECT version FROM successes ORDER BY id DESC LIMIT 1
		),
		compatibility AS (
			-- strictest declaration among steps at or below current -- a step
			-- rolled back below current no longer binds. The current row itself
			-- qualifies, so MAX never aggregates an empty set
			SELECT MAX(successes.min_compatible_version) AS min_compatible_version
			FROM successes, current
			WHERE successes.version <= current.version
		)
		SELECT current.version, compatibility.min_compatible_version
		FROM current, compatibility;
	`, d.Datastore.Schema)

	var state SchemaStateRow
	if err := q.QueryRow(ctx, sql, systemId).Scan(&state.Version, &state.MinCompatibleVersion); err != nil {
		return nil, registrationError(err)
	}
	return &state, nil
}

// TopicSchemaState is a topic's version facts, read on the pool: current from
// the latest-by-id success row, minimum compatible from the strictest step at
// or below it.
func (d *MigrateDatastore) TopicSchemaState(ctx context.Context, topicId int64) (*SchemaStateRow, error) {
	var state *SchemaStateRow
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		state, err = d.topicSchemaState(ctx, d.Datastore.Pool, topicId)
		return err
	})
	return state, err
}

// TopicSchemaStateInTx reads the topic's version facts through tx.
func (d *MigrateDatastore) TopicSchemaStateInTx(ctx context.Context, tx datastore.Tx, topicId int64) (*SchemaStateRow, error) {
	return d.topicSchemaState(ctx, tx, topicId)
}

func (d *MigrateDatastore) topicSchemaState(ctx context.Context, q datastore.Querier, topicId int64) (*SchemaStateRow, error) {
	sql := fmt.Sprintf(`
		-- vulkan: migrate.topicSchemaState
		WITH successes AS (
			SELECT id, version, min_compatible_version
			FROM %[1]s.migration_log
			WHERE system_id IS NULL
				AND topic_id = $1
				AND consumer_group_id IS NULL
				AND status = 'success'
		),
		current AS (
			-- latest-by-id, not MAX -- a downgrade records a lower version
			SELECT version FROM successes ORDER BY id DESC LIMIT 1
		),
		compatibility AS (
			-- strictest declaration among steps at or below current -- a step
			-- rolled back below current no longer binds. The current row itself
			-- qualifies, so MAX never aggregates an empty set
			SELECT MAX(successes.min_compatible_version) AS min_compatible_version
			FROM successes, current
			WHERE successes.version <= current.version
		)
		SELECT current.version, compatibility.min_compatible_version
		FROM current, compatibility;
	`, d.Datastore.Schema)

	var state SchemaStateRow
	if err := q.QueryRow(ctx, sql, topicId).Scan(&state.Version, &state.MinCompatibleVersion); err != nil {
		return nil, registrationError(err)
	}
	return &state, nil
}

// Version is an owner's latest-by-id success row -- latest-by-id, NOT MAX, so
// a downgrade (which records a LOWER version) reads back correctly. The
// migration run is owner-generic (one steps loop, one record insert for every
// scope), so its read on the lock-holding connection is too.
//
// There is no implied baseline but every owner is recorded at creation.
func Version(ctx context.Context, q datastore.Querier, owner *common.Owner, schema string) (int64, error) {
	// IS NOT DISTINCT FROM: NULL-safe equality against the owner's columns
	sql := fmt.Sprintf(`
		-- vulkan: migrate.Version
		SELECT version FROM %[1]s.migration_log
		WHERE system_id IS NOT DISTINCT FROM $1
			AND topic_id IS NOT DISTINCT FROM $2
			AND consumer_group_id IS NOT DISTINCT FROM $3
			AND status = 'success'
		ORDER BY id DESC
		LIMIT 1;
	`, schema)

	var version int64
	if err := q.QueryRow(ctx, sql, owner.SystemIdColumn(), owner.TopicIdColumn(), owner.ConsumerGroupIdColumn()).Scan(&version); err != nil {
		return 0, registrationError(err)
	}
	return version, nil
}
