package schedule

import (
	"encoding/json"
	"regexp"
	"time"

	"github.com/agentstax/sqlstreams/pkg/common"
)

// a schedule's name doubles as the message key and routing key its messages are produced with,
// so it can't contain '*' -- the binding wildcard, which a pattern can't
// escape
var SlugPattern = regexp.MustCompile(`^[a-z0-9._-]+$`)

// Schedule is one row of schedule_config joined to its schedule_cursor row.
// Every schedule is the system's; StreamId is the target stream every produce
// lands on.
type Schedule struct {
	Id              int64                    `json:"schedule_id"`
	SystemId        int64                    `json:"system_id"`
	StreamId        int64                    `json:"stream_id"`
	Name            string                   `json:"schedule"`
	Expression      string                   `json:"expression"`     // the cron expression
	SchemaVersion   int                      `json:"schema_version"` // the payload type's declared version
	Concurrency     common.ConcurrencyPolicy `json:"concurrency"`    // how each produced message runs, with Timeout
	Timeout         time.Duration            `json:"timeout"`
	Suspended       bool                     `json:"suspended"`         // nothing is produced until Unsuspend
	Payload         json.RawMessage          `json:"payload"`           // produced as-is on every run
	Metadata        json.RawMessage          `json:"metadata"`          // opaque; {} when none was declared
	NextScheduledAt time.Time                `json:"next_scheduled_at"` // the scheduled time the next produce is for
	LastScheduledAt *time.Time               `json:"last_scheduled_at"` // nil until the first produce
}
