package record

import "time"

// BacklogRecord is one second of one consumer group's position: the highest
// message id the topic holds and the group's committed cursor. Their
// difference is the group's backlog.
type BacklogRecord struct {
	At             time.Time `json:"at"`
	Topic          string    `json:"topic"`
	Group          string    `json:"group"`
	HighestMessage int64     `json:"highest_message"`
	Committed      int64     `json:"committed"`
}
