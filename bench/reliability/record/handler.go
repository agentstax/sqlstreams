package record

import "time"

type HandlerOutcome string

const (
	HandlerSuccess HandlerOutcome = "success"
	HandlerError   HandlerOutcome = "error"
)

// Handler is one row of handler_ledger: one handler invocation, written
// before the handler returns so a crash after it still leaves the row.
type Handler struct {
	At        time.Time      `json:"at"`
	Consumer  string         `json:"consumer"`
	Group     string         `json:"group"`
	MessageId int64          `json:"message_id"`
	Key       string         `json:"key"`
	Attempt   int            `json:"attempt"`
	Outcome   HandlerOutcome `json:"outcome"`
}
