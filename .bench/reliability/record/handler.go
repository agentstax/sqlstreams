package record

import "time"

type HandlerOutcome string

const (
	HandlerOutcomeSuccess HandlerOutcome = "success"
	HandlerOutcomeError   HandlerOutcome = "error"
)

// HandlerRecord is one row of handler_record: one handler invocation, written
// before the handler returns so a crash after it still leaves the row.
type HandlerRecord struct {
	At        time.Time      `json:"at"`
	Consumer  string         `json:"consumer"`
	Topic     string         `json:"topic"`
	Group     string         `json:"group"`
	MessageId int64          `json:"message_id"`
	Key       string         `json:"key"`
	Attempt   int            `json:"attempt"`
	Outcome   HandlerOutcome `json:"outcome"`
}
