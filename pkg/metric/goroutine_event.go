package metric

import "time"

type GoRoutineEvent struct {
	EventType EventType `json:"type"`
	StreamId  int64     `json:"stream_id"`
	Group     string    `json:"group"`
	MessageId int64     `json:"message_id"`
	Attempt   int       `json:"attempt"`
	At        time.Time `json:"at"` // needed for accurate timing with draining behavior
}

func (GoRoutineEvent) SchemaVersion() int { return 1 }

func NewGoRoutineEvent(eventType EventType, streamId int64, group string, messageId int64, attempt int, at time.Time) *GoRoutineEvent {
	return &GoRoutineEvent{
		EventType: eventType,
		StreamId:  streamId,
		Group:     group,
		MessageId: messageId,
		Attempt:   attempt,
		At:        at,
	}
}
