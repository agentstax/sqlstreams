package record

import "time"

// BacklogRecord is one second of one consumer group's position: the highest
// message id the stream holds and the group's committed cursor. Their
// difference is the group's backlog.
type BacklogRecord struct {
	At             time.Time `json:"at"`
	Stream         string    `json:"stream"`
	Group          string    `json:"group"`
	HighestMessage int64     `json:"highest_message"`
	Committed      int64     `json:"committed"`
}
