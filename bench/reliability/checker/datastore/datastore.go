package datastore

// datastore is every query the checker runs: the lab schema the record
// files are loaded into, and the reads across it and vulkan's own tables.
// The judgment lives in package checker; nothing here decides.

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CheckerDatastore struct {
	pool *pgxpool.Pool
}

func NewCheckerDatastore(pool *pgxpool.Pool) (*CheckerDatastore, error) {
	if pool == nil {
		return nil, errors.New("pool must not be nil")
	}
	return &CheckerDatastore{pool: pool}, nil
}

// ReadSynchronousCommit is the server setting the verdict records: a run with
// it off proves less about durability than one with it on.
func (d *CheckerDatastore) ReadSynchronousCommit(ctx context.Context) (string, error) {
	settingSql := `
		-- lab: datastore.ReadSynchronousCommit
		SHOW synchronous_commit;
	`
	var setting string
	err := d.pool.QueryRow(ctx, settingSql).Scan(&setting)
	return setting, err
}
