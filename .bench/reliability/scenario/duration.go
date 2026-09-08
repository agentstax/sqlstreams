package scenario

import (
	"fmt"
	"time"
)

// formatDuration prints the way a scenario is written: whole minutes as
// "10m", whole seconds as "30s", anything finer as Go prints it.
func formatDuration(duration time.Duration) string {
	if duration%time.Minute == 0 {
		return fmt.Sprintf("%dm", int(duration/time.Minute))
	}
	if duration%time.Second == 0 {
		return fmt.Sprintf("%ds", int(duration/time.Second))
	}
	return duration.String()
}
