package checker

import (
	"context"
	"fmt"
	"time"

	"github.com/agentstax/sqlstreams/.bench/checker/datastore"
	"github.com/agentstax/sqlstreams/.bench/common"
)

const drainPoll = 500 * time.Millisecond

// drain waits until the group's cursor has passed the highest message the
// stream holds -- not the records' last committed id, so a recovered or
// unexpected row above it is settled too -- and the checks read finished
// work, not work in flight. The budget spent is a verdict of unknown: the
// checker cannot tell a slow consumer from a stuck one.
func (c *Checker) drain(ctx context.Context, target datastore.Target) error {
	deadline := time.Now().Add(c.drainBudget)
	for {
		position, err := c.ds.ReadDrainPosition(ctx, target)
		if err != nil {
			return err
		}
		if position.CursorCommitted >= position.HighestMessage {
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

// settleProgress reloads the progress snapshots until every consumer
// series has one taken after the drain completed: a handler's counter
// moves before its delivery commits, so a snapshot after the last cursor
// advance holds the final total. The consumers snapshot once a second and
// keep running until the manager stops them, after this checker exits.
// Only aggregate-recording runs write snapshots, so only they settle.
func (c *Checker) settleProgress(ctx context.Context) (int64, error) {
	drainedAt := time.Now()
	deadline := drainedAt.Add(c.drainBudget)
	for {
		loaded, err := c.ds.ReloadProgress(ctx, c.recordDir)
		if err != nil {
			return 0, err
		}
		oldest, err := c.ds.ReadOldestHandlerSnapshot(ctx)
		if err != nil {
			return 0, err
		}
		if !oldest.Before(drainedAt) {
			return loaded, nil
		}
		if time.Now().After(deadline) {
			return 0, fmt.Errorf("drain budget %v spent waiting for a progress snapshot after the drain: oldest latest snapshot %v, drained at %v", c.drainBudget, oldest.Format(time.RFC3339Nano), drainedAt.Format(time.RFC3339Nano))
		}
		if err := common.WaitUntil(ctx, time.Now().Add(drainPoll)); err != nil {
			return 0, err
		}
	}
}
