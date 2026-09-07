package checker

import (
	"context"
	"fmt"
	"time"

	"github.com/agentstax/vulkan/bench/reliability/lab"
)

const drainPoll = 500 * time.Millisecond

// drainPosition is the two ids drain compares: the highest message id the
// ledger saw committed, and the group's cursor -- every id at or below
// `committed` is done or dead.
type drainPosition struct {
	LastCommitted   int64
	CursorCommitted int64
}

func (p drainPosition) drained() bool {
	return p.CursorCommitted >= p.LastCommitted
}

// drain waits until the group's cursor has passed the last committed
// message, so the checks read finished work and not work in flight. The
// budget spent is a verdict of unknown: the checker cannot tell a slow
// consumer from a stuck one.
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
			return fmt.Errorf("drain budget %v spent: cursor committed %d, last committed message %d", c.drainBudget, position.CursorCommitted, position.LastCommitted)
		}
		if err := lab.WaitUntil(ctx, time.Now().Add(drainPoll)); err != nil {
			return err
		}
	}
}

func (c *Checker) readDrainPosition(ctx context.Context, target *target) (drainPosition, error) {
	positionSql := fmt.Sprintf(`
		-- lab: checker.readDrainPosition
		SELECT
			(SELECT COALESCE(max(message_id), 0) FROM %[1]s WHERE kind = 'committed'),
			(SELECT COALESCE(max(committed), 0) FROM %[2]s WHERE consumer_group_id = $1);
	`, produceLedger, target.consumerGroupCursor())
	var position drainPosition
	err := c.pool.QueryRow(ctx, positionSql, target.groupId).Scan(&position.LastCommitted, &position.CursorCommitted)
	return position, err
}
