package datastore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ServerDeltas is the server's counters over one window: the difference
// between the observer sample nearest each bound, whose instants are
// recorded so the coverage is visible.
type ServerDeltas struct {
	From        time.Time `json:"from"`
	To          time.Time `json:"to"`
	WalRecords  int64     `json:"wal_records"`
	WalFpi      int64     `json:"wal_fpi"`
	WalBytes    int64     `json:"wal_bytes"`
	Checkpoints int64     `json:"checkpoints"`
	XactCommit  int64     `json:"xact_commit"`
	Deadlocks   int64     `json:"deadlocks"`
	BlocksHit   int64     `json:"blocks_hit"`
	BlocksRead  int64     `json:"blocks_read"`
}

func (d *CheckerDatastore) ReadServerDeltas(ctx context.Context, from time.Time, to time.Time) (ServerDeltas, error) {
	deltasSql := fmt.Sprintf(`
		-- lab: datastore.ReadServerDeltas
		WITH first AS (
			SELECT
				at,
				wal_records,
				wal_fpi,
				wal_bytes,
				checkpoints,
				xact_commit,
				deadlocks,
				blocks_hit,
				blocks_read
			FROM %[1]s
			ORDER BY abs(EXTRACT(EPOCH FROM (at - $1::timestamptz)))
			LIMIT 1
		), last AS (
			SELECT
				at,
				wal_records,
				wal_fpi,
				wal_bytes,
				checkpoints,
				xact_commit,
				deadlocks,
				blocks_hit,
				blocks_read
			FROM %[1]s
			ORDER BY abs(EXTRACT(EPOCH FROM (at - $2::timestamptz)))
			LIMIT 1
		)
		SELECT
			first.at,
			last.at,
			last.wal_records - first.wal_records,
			last.wal_fpi - first.wal_fpi,
			last.wal_bytes - first.wal_bytes,
			last.checkpoints - first.checkpoints,
			last.xact_commit - first.xact_commit,
			last.deadlocks - first.deadlocks,
			last.blocks_hit - first.blocks_hit,
			last.blocks_read - first.blocks_read
		FROM first, last;
	`, observerSample)
	var deltas ServerDeltas
	err := d.pool.QueryRow(ctx, deltasSql, from, to).Scan(
		&deltas.From, &deltas.To, &deltas.WalRecords, &deltas.WalFpi, &deltas.WalBytes, &deltas.Checkpoints,
		&deltas.XactCommit, &deltas.Deadlocks, &deltas.BlocksHit, &deltas.BlocksRead)
	if errors.Is(err, pgx.ErrNoRows) {
		return ServerDeltas{}, errors.New("no observer samples -- the observer role did not run")
	}
	return deltas, err
}
