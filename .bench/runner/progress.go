package runner

import (
	"context"
	"time"

	"github.com/allegedlyreliable/sqlstreams/.bench/record"
)

// runProgress writes cumulative counters once a second and once more at shutdown.
func runProgress(ctx context.Context, progress []*record.Progress) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		for _, counters := range progress {
			if err := counters.Snapshot(); err != nil {
				return err
			}
		}
		select {
		case <-ctx.Done():
			for _, counters := range progress {
				if err := counters.Snapshot(); err != nil {
					return err
				}
			}
			return nil
		case <-ticker.C:
		}
	}
}
