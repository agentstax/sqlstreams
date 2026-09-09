package datastore

import (
	"context"
	"fmt"
)

// DrainPosition is the two ids the drain compares: the highest message id
// the stream holds, and the group's cursor -- every id at or below committed
// is done or dead.
type DrainPosition struct {
	HighestMessage  int64
	CursorCommitted int64
}

func (d *CheckerDatastore) ReadDrainPosition(ctx context.Context, target Target) (DrainPosition, error) {
	positionSql := fmt.Sprintf(`
		-- lab: datastore.ReadDrainPosition
		SELECT
			(SELECT COALESCE(max(id), 0) FROM %[1]s),
			(SELECT COALESCE(max(committed), 0) FROM %[2]s WHERE consumer_group_id = $1);
	`, target.messageLog(), target.consumerGroupCursor())
	var position DrainPosition
	err := d.pool.QueryRow(ctx, positionSql, target.GroupId).Scan(&position.HighestMessage, &position.CursorCommitted)
	return position, err
}
