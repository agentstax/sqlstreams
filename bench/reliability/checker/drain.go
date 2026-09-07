package checker

import (
	"context"
	"fmt"
	"time"

	"github.com/agentstax/vulkan/bench/reliability/common"
)

const drainPoll = 500 * time.Millisecond

// drainPosition is the two ids drain compares: the highest message id the
// topic holds, and the group's cursor -- every id at or below `committed`
// is done or dead.
type drainPosition struct {
	HighestMessage  int64
	CursorCommitted int64
}

func (p drainPosition) drained() bool {
	return p.CursorCommitted >= p.HighestMessage
}

// drain waits until the group's cursor has passed the highest message the
// topic holds -- not the ledger's last committed id, so a recovered or
// unexpected row above it is settled too -- and the checks read finished
// work, not work in flight. The budget spent is a verdict of unknown: the
// checker cannot tell a slow consumer from a stuck one.
func (c *Checker) drain(ctx context.Context, target *target) error {
	deadline := time.Now().Add(c.drainBudget)
	for {
		position, err := c.readDrainPosition(ctx, target)
		if err != nil {
			return err
		}
		if position.drained() {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("drain budget %v spent: cursor committed %d, highest message %d", c.drainBudget, position.CursorCommitted, position.HighestMessage)
		}
		if err := common.WaitUntil(ctx, time.Now().Add(drainPoll)); err != nil {
			return err
		}
	}
}

func (c *Checker) readDrainPosition(ctx context.Context, target *target) (drainPosition, error) {
	positionSql := fmt.Sprintf(`
		-- lab: checker.readDrainPosition
		SELECT
			(SELECT COALESCE(max(id), 0) FROM %[1]s),
			(SELECT COALESCE(max(committed), 0) FROM %[2]s WHERE consumer_group_id = $1);
	`, target.messageLog(), target.consumerGroupCursor())
	var position drainPosition
	err := c.pool.QueryRow(ctx, positionSql, target.groupId).Scan(&position.HighestMessage, &position.CursorCommitted)
	return position, err
}
