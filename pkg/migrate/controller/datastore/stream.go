package datastore

import (
	"context"
	"fmt"

	"github.com/allegedlyreliable/sqlstreams/pkg/common"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (d *MigrateDatastore) ListStreams(ctx context.Context, conn *pgxpool.Conn) ([]*common.Owner, error) {
	sql := fmt.Sprintf(`
		-- sqlstreams: migrate.ListStreams
		SELECT id, system_id, name FROM %[1]s.stream_config ORDER BY id;
	`, d.Datastore.Schema)
	rows, err := conn.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var streams []*common.Owner
	for rows.Next() {
		var id int64
		var systemId int64
		var name string
		if err := rows.Scan(&id, &systemId, &name); err != nil {
			return nil, err
		}
		owner, err := common.NewStreamOwner(systemId, id, name)
		if err != nil {
			return nil, err
		}
		streams = append(streams, owner)
	}
	return streams, rows.Err()
}
