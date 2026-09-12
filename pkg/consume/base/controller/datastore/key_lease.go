package datastore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/datastore"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Claim attempts to acquire the exclusive right to run a keyed
// message. For a compacted message, Acquired also guarantees it was still
// its key's compaction head after the lease was won; an uncompacted message
// is never superseded, so only the lease itself is contested.
// Expiry does not stop a holder: the next claim on the key takes the lease
// over, and the two runs can overlap until the old one returns.
func (d *KeyLeaseDatastore) Claim(ctx context.Context, streamId int64, groupId int64, key string, messageId int64, compacted bool, policy common.ConcurrencyPolicy, ownLow int64, ownHigh int64, duration time.Duration, token pgtype.UUID) (*KeyLease, error) {
	var claim *KeyLease
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		claim, err = d.claim(ctx, streamId, groupId, key, messageId, compacted, policy, ownLow, ownHigh, duration, token)
		return err
	})
	return claim, err
}

func (d *KeyLeaseDatastore) claim(ctx context.Context, streamId int64, groupId int64, key string, messageId int64, compacted bool, policy common.ConcurrencyPolicy, ownLow int64, ownHigh int64, duration time.Duration, token pgtype.UUID) (*KeyLease, error) {
	switch {
	case compacted:
		return d.claimCompacted(ctx, streamId, groupId, key, messageId, duration, token)
	case policy == common.ConcurrencyOrdered:
		return d.claimOrdered(ctx, streamId, groupId, key, messageId, ownLow, ownHigh, duration, token)
	default:
		return d.claimUncompacted(ctx, streamId, groupId, key, duration, token)
	}
}

// claimCompacted gates the lease on the key's compaction head: a superseded
// message never creates or locks a lease row.
func (d *KeyLeaseDatastore) claimCompacted(ctx context.Context, streamId int64, groupId int64, key string, messageId int64, duration time.Duration, token pgtype.UUID) (*KeyLease, error) {
	claimSql := fmt.Sprintf(`
		-- sqlstreams: consumebase.claimCompacted
		WITH head AS (
			SELECT message_id
			FROM %[1]s.%[2]s
			WHERE compaction_key = $1
				AND message_id IS NOT NULL
		), attempt AS (
			INSERT INTO %[1]s.%[3]s AS kl (consumer_group_id, message_key, token, expires_at)
			SELECT $2, $1, $5, now() + make_interval(secs => $4)
			WHERE EXISTS (SELECT 1 FROM head WHERE message_id = $3)
			ON CONFLICT (consumer_group_id, message_key) DO UPDATE
			SET
				token = $5,
				expires_at = now() + make_interval(secs => $4)
			-- the token match lets a retry after an ambiguous commit re-take its
			-- own lease instead of reading it as busy
			WHERE kl.expires_at < now() OR kl.token = $5
			RETURNING token
		)
		SELECT
			EXISTS (SELECT 1 FROM head WHERE message_id = $3),
			(SELECT token FROM attempt);
	`, d.Datastore.Schema, stream.CompactionHeadTable(streamId), stream.MessageKeyLeaseTable(streamId))

	// the claimSql head CTE snapshot could be stale on the INSERT that
	// follows -- this rechecks with a fresh snapshot and deletes the
	// acquisition if the head moved. So a failed batch rolls the insert
	// back instead of orphaning the lease.
	recheckSql := fmt.Sprintf(`
		-- sqlstreams: consumebase.claimCompacted
		DELETE FROM %[1]s.%[2]s
		WHERE consumer_group_id = $2
			AND message_key = $1
			AND token = $4
			AND NOT EXISTS (
				SELECT 1
				FROM %[1]s.%[3]s
				WHERE compaction_key = $1
					AND message_id = $3
			);
	`, d.Datastore.Schema, stream.MessageKeyLeaseTable(streamId), stream.CompactionHeadTable(streamId))

	// one round trip
	batch := &pgx.Batch{}
	batch.Queue(claimSql, key, groupId, messageId, duration.Seconds(), token)
	batch.Queue(recheckSql, key, groupId, messageId, token)

	claim := KeyLease{StreamId: streamId, ConsumerGroupId: groupId, MessageKey: key}
	var isHead bool

	// claimSql
	results := d.Datastore.Pool.SendBatch(ctx, batch)
	if err := results.QueryRow().Scan(&isHead, &claim.Token); err != nil {
		results.Close()
		return nil, err
	}

	// recheckSql
	recheckTag, err := results.Exec()
	if err != nil {
		results.Close()
		return nil, err
	}
	if err := results.Close(); err != nil {
		return nil, err
	}

	switch {
	case !isHead:
		claim.Verdict = KeyLeaseSuperseded
	case !claim.Token.Valid:
		claim.Verdict = KeyLeaseBusy
	case recheckTag.RowsAffected() > 0:
		// the head moved mid-claim -- the newer version must run, not this one
		claim.Verdict = KeyLeaseSuperseded
		claim.Token = pgtype.UUID{}
	default:
		claim.Verdict = KeyLeaseAcquired
	}
	return &claim, nil
}

// claimUncompacted takes the lease with no compaction head to check: every
// version of the key runs, so the only verdicts are acquired and busy.
func (d *KeyLeaseDatastore) claimUncompacted(ctx context.Context, streamId int64, groupId int64, key string, duration time.Duration, token pgtype.UUID) (*KeyLease, error) {
	sql := fmt.Sprintf(`
		-- sqlstreams: consumebase.claimUncompacted
		INSERT INTO %[1]s.%[2]s AS kl (consumer_group_id, message_key, token, expires_at)
		VALUES ($1, $2, $3, now() + make_interval(secs => $4))
		ON CONFLICT (consumer_group_id, message_key) DO UPDATE
		SET
			token = $3,
			expires_at = now() + make_interval(secs => $4)
		-- the token match lets a retry after an ambiguous commit re-take its
		-- own lease instead of reading it as busy
		WHERE kl.expires_at < now() OR kl.token = $3
		RETURNING token;
	`, d.Datastore.Schema, stream.MessageKeyLeaseTable(streamId))

	claim := KeyLease{StreamId: streamId, ConsumerGroupId: groupId, MessageKey: key}
	err := d.Datastore.Pool.QueryRow(ctx, sql, groupId, key, token, duration.Seconds()).Scan(&claim.Token)
	if errors.Is(err, pgx.ErrNoRows) {
		claim.Verdict = KeyLeaseBusy
		return &claim, nil
	}
	if err != nil {
		return nil, err
	}

	claim.Verdict = KeyLeaseAcquired
	return &claim, nil
}

// claimOrdered takes the lease only when:
//   - no earlier same-key message is unresolved for the group
//   - no exception row still ready/inflight/deferred
//   - no same-key message_log id between the group's committed cursor and this message,
//     outside the caller's own range (ownLow, ownHigh] -- the caller runs those in order
func (d *KeyLeaseDatastore) claimOrdered(ctx context.Context, streamId int64, groupId int64, key string, messageId int64, ownLow int64, ownHigh int64, duration time.Duration, token pgtype.UUID) (*KeyLease, error) {
	sql := fmt.Sprintf(`
		-- sqlstreams: consumebase.claimOrdered
		INSERT INTO %[1]s.%[2]s AS kl (consumer_group_id, message_key, token, expires_at)
		SELECT $1, $2, $3, now() + make_interval(secs => $4)
		-- no exception row still ready/inflight/deferred
		WHERE NOT EXISTS (
			SELECT 1
			FROM %[1]s.%[3]s earlier
			WHERE earlier.consumer_group_id = $1
				AND earlier.message_key = $2
				AND earlier.message_id < $5
				AND earlier.status IN ('ready', 'inflight', 'deferred')
		)
		-- no same-key message_log id between the group's committed cursor and this message,
		-- outside the caller's own range (ownLow, ownHigh] -- the caller runs those in order
		AND NOT EXISTS (
			SELECT 1
			FROM %[1]s.%[4]s m
			WHERE m.message_key = $2
				AND m.id < $5
				AND m.id > (SELECT committed FROM %[1]s.%[5]s WHERE consumer_group_id = $1)
				AND NOT (m.id > $6 AND m.id <= $7)
		)
		ON CONFLICT (consumer_group_id, message_key) DO UPDATE
		SET
			token = $3,
			expires_at = now() + make_interval(secs => $4)
		-- the token match lets a retry after an ambiguous commit re-take its
		-- own lease instead of reading it as busy
		WHERE kl.expires_at < now() OR kl.token = $3
		RETURNING token;
	`, d.Datastore.Schema, stream.MessageKeyLeaseTable(streamId), stream.ExceptionQueueTable(streamId), stream.MessageLogTable(streamId), stream.ConsumerGroupCursorTable(streamId))

	claim := KeyLease{StreamId: streamId, ConsumerGroupId: groupId, MessageKey: key}
	err := d.Datastore.Pool.QueryRow(ctx, sql, groupId, key, token, duration.Seconds(), messageId, ownLow, ownHigh).Scan(&claim.Token)
	if errors.Is(err, pgx.ErrNoRows) {
		claim.Verdict = KeyLeaseBusy
		return &claim, nil
	}
	if err != nil {
		return nil, err
	}

	claim.Verdict = KeyLeaseAcquired
	return &claim, nil
}

// Release frees an acquired key.
// false -> no row matched the claim's Token: the lease expired, and the
// row was taken over or deleted by the janitor.
func (d *KeyLeaseDatastore) Release(ctx context.Context, claim *KeyLease) (bool, error) {
	var released bool
	err := d.DatastoreRetry.Wrap(ctx, func() error {
		var err error
		released, err = d.release(ctx, d.Datastore.Pool, claim)
		return err
	})
	return released, err
}

func (d *KeyLeaseDatastore) release(ctx context.Context, q datastore.Querier, claim *KeyLease) (bool, error) {
	sql := fmt.Sprintf(`
		-- sqlstreams: consumebase.release
		DELETE FROM %[1]s.%[2]s
		WHERE consumer_group_id = $1
			AND message_key = $2
			AND token = $3;
	`, d.Datastore.Schema, stream.MessageKeyLeaseTable(claim.StreamId))
	tag, err := q.Exec(ctx, sql, claim.ConsumerGroupId, claim.MessageKey, claim.Token)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
