package record

import "time"

// BacklogRecord estimates outstanding ids from the allocation sequence and durable cursor.
// HighestMessage is retained for reading older records that queried message rows.
type BacklogRecord struct {
	At               time.Time `json:"at"`
	Stream           string    `json:"stream"`
	Group            string    `json:"group"`
	HighestAllocated int64     `json:"highest_allocated,omitempty"`
	HighestMessage   int64     `json:"highest_message"`
	Committed        int64     `json:"committed"`
}
