package datastore

// datastore is every query the checker runs: the lab schema the record
// files are loaded into, and the reads across it and sqlstreams's own tables.
// The judgment lives in package checker; nothing here decides.

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CheckerDatastore struct {
	pool   *pgxpool.Pool
	Config *CheckerDatastoreConfig
}

func NewCheckerDatastore(pool *pgxpool.Pool, cfg *CheckerDatastoreConfig) (*CheckerDatastore, error) {
	if pool == nil {
		return nil, errors.New("pool must not be nil")
	}
	if cfg == nil {
		cfg = &CheckerDatastoreConfig{}
	}
	cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &CheckerDatastore{pool: pool, Config: cfg}, nil
}

// ReadServerVersion is the version the server reports, recorded in the
// verdict beside the image tag compose asked for.
func (d *CheckerDatastore) ReadServerVersion(ctx context.Context) (string, error) {
	versionSql := `
		-- lab: datastore.ReadServerVersion
		SHOW server_version;
	`
	var version string
	err := d.pool.QueryRow(ctx, versionSql).Scan(&version)
	return version, err
}

// ReadSettings reads the named settings as the server reports them, in
// SHOW's units, so the verdict carries what ran rather than what compose
// asked for.
func (d *CheckerDatastore) ReadSettings(ctx context.Context, names []string) (map[string]string, error) {
	settingsSql := `
		-- lab: datastore.ReadSettings
		SELECT name, current_setting(name)
		FROM unnest($1::text[]) AS name;
	`
	rows, err := d.pool.Query(ctx, settingsSql, names)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := map[string]string{}
	for rows.Next() {
		var name, setting string
		if err := rows.Scan(&name, &setting); err != nil {
			return nil, err
		}
		settings[name] = setting
	}
	return settings, rows.Err()
}
