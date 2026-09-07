package checker

import "context"

// synchronousCommit is the server setting the verdict records: a run with
// it off proves less about durability than one with it on.
func (c *Checker) synchronousCommit(ctx context.Context) (string, error) {
	var setting string
	err := c.pool.QueryRow(ctx, "SHOW synchronous_commit;").Scan(&setting)
	return setting, err
}
