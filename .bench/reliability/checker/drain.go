package checker

import (
	"context"
	"fmt"
	"time"

	"github.com/agentstax/vulkan/.bench/reliability/checker/datastore"
	"github.com/agentstax/vulkan/.bench/reliability/common"
)

const drainPoll = 500 * time.Millisecond

// drain waits until the group's cursor has passed the highest message the
// topic holds -- not the records' last committed id, so a recovered or
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
