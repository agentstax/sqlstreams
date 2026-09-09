package consume

import "time"

// Consumer is the registered consumer group row. A group is owned by
// exactly one stream -- names are unique per
// stream, not globally. Children (cursor, lease, binding) reference Id and
// carry no stream_id of their own; the stream_id FK cascade is the group's
// lifecycle -- destroying the stream destroys it.
type Consumer struct {
	Id        int64     `json:"group_id"`
	StreamId  int64     `json:"stream_id"`
	Name      string    `json:"group"`
	CreatedAt time.Time `json:"created_at"`
}
