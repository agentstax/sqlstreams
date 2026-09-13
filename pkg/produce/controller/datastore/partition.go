package datastore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/allegedlyreliable/sqlstreams/pkg/produce"
	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ddlLockTimeout caps how long the self-heal CREATE waits for its table lock.
// WAITING is the hazard, not failing: postgres queues every later produce and
// claim behind the lock, so a stuck lock holder would stall the whole stream --
// better to fail this produce fast and let the caller's retry policy own it.
const ddlLockTimeout = 2 * time.Second

// createAheadAttemptAllowance is one attempt's share of the create-ahead
// timeout: two lock_timeout-bounded waits (advisory lock, CREATE) plus
// round-trip slack.
const createAheadAttemptAllowance = 3 * ddlLockTimeout

// insertUntilCovered runs insert until a partition covers it. An insert
// learns its ids from the sequence, so a rerun can land past the partition
// the previous heal created; each heal covers the sequence's next id, so
// the loop only continues while other producers advanced the sequence a
// whole partition in between. One heal for the boundary itself, then one
// per configured retry.
func (d *ProduceDatastore) insertUntilCovered(ctx context.Context, streamId int64, partitionSize int64, insert func() error) error {
	for heals := 0; ; heals++ {
		err := insert()
		if !isMissingPartition(err) {
			return err
		}
		if heals > d.DatastoreRetry.MaxRetries {
			return produce.ErrPartitionCreationBehind.Wrap(err).With("stream_id", streamId)
		}

		if err := d.createNextIdPartition(ctx, streamId, partitionSize); err != nil {
			return err
		}
	}
}

// createNextIdPartition creates the partition the next id will land in.
// can't use the passed id as that id is already likely burned from an
// attempt in the sequence table.
func (d *ProduceDatastore) createNextIdPartition(ctx context.Context, streamId int64, partitionSize int64) error {
	lastValueSql := fmt.Sprintf(`
		-- sqlstreams: produce.createNextIdPartition
		SELECT last_value FROM %[1]s.%[2]s;
	`, d.Datastore.Schema, stream.MessageLogIdSequence(streamId))

	var lastValue int64
	if err := d.Datastore.Pool.QueryRow(ctx, lastValueSql).Scan(&lastValue); err != nil {
		return err
	}

	next := lastValue + 1
	d.Logger.WarnContext(ctx, produce.EventPartitionCreatedOnInsert.Message(), "code", produce.EventPartitionCreatedOnInsert.GetCode(), "stream_id", streamId, "message_id", next)

	return d.ensureCoveringPartition(ctx, streamId, partitionSize, next)
}

// ensureCoveringPartition creates the partition that covers id.
func (d *ProduceDatastore) ensureCoveringPartition(ctx context.Context, streamId int64, partitionSize int64, id int64) error {
	next := id / partitionSize

	createPartitionSql := fmt.Sprintf(`
		-- sqlstreams: produce.ensureCoveringPartition
		CREATE TABLE IF NOT EXISTS %[1]s.%[2]s
			PARTITION OF %[1]s.%[3]s
			FOR VALUES FROM (%[4]d) TO (%[5]d);
	`, d.Datastore.Schema, stream.MessageLogPartitionTable(streamId, next), stream.MessageLogTable(streamId), next*partitionSize, (next+1)*partitionSize)

	lockKey, err := common.NewAdvisoryLockKey("partition", d.Datastore.Schema, streamId, next)
	if err != nil {
		return err
	}

	tx, err := d.Datastore.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// SET LOCAL scopes the cap to this transaction: a session-level SET would
	// leak to whatever uses this pooled connection next
	lockTimeoutSql := fmt.Sprintf(`
		-- sqlstreams: produce.ensureCoveringPartition
		SET LOCAL lock_timeout = '%dms';
	`, ddlLockTimeout.Milliseconds())
	if _, err := tx.Exec(ctx, lockTimeoutSql); err != nil {
		return err
	}

	// one winner runs the CREATE; every concurrent caller sleeps here (bounded
	// by the lock_timeout above) until that commit releases the lock.
	advisoryLockSql := `
		-- sqlstreams: produce.ensureCoveringPartition
		SELECT pg_advisory_xact_lock($1);
	`
	if _, err := tx.Exec(ctx, advisoryLockSql, lockKey.Value()); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, createPartitionSql); err != nil {
		// IF NOT EXISTS still races -- losing to a concurrent creator means it exists
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "42P07" {
			return nil
		}
		return err
	}
	return tx.Commit(ctx)
}

// createPartitionAhead creates the partition after id's early, in the
// background. Best-effort: a failure warns and drops.
func (d *ProduceDatastore) createPartitionAhead(streamId int64, partitionSize int64, id int64) {
	next := (id/partitionSize + 1) * partitionSize

	go func() {
		// the produce ctx dies with its caller, so the run carries its own
		ctx, cancel := context.WithTimeout(context.Background(), d.createAheadTimeout)
		defer cancel()

		err := d.DatastoreRetry.WrapIdempotent(ctx, func() error {
			err := d.ensureCoveringPartition(ctx, streamId, partitionSize, next)
			if isLockNotAvailable(err) {
				return produce.ErrPartitionLockTimeout.Wrap(err)
			}
			return err
		})
		if err != nil {
			// a missing parent table means the stream was destroyed while this
			// goroutine was in flight -- drop its claim entry
			if isMissingTable(err) {
				d.createAheadGate.delete(streamId)
				return
			}
			d.Logger.WarnContext(ctx, produce.EventPartitionNotCreatedAhead.Message(), "code", produce.EventPartitionNotCreatedAhead.GetCode(), "stream_id", streamId, "error", err)
		}
	}()
}

// ***************
// *** HELPERS ***
// ***************

// isMissingPartition matches an insert routed to a partition that doesn't exist yet.
func isMissingPartition(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == "23514" && // check_violation doubles as partition-routing failure
		strings.Contains(pgErr.Message, "no partition of relation")
}

// isMissingTable matches a statement against a table that no longer exists.
func isMissingTable(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42P01" // undefined_table
}

// isLockNotAvailable matches a lock_timeout expiry.
func isLockNotAvailable(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "55P03" // lock_not_available
}
