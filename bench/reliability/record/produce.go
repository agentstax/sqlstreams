package record

import "time"

// ProduceKind is which of a produce's two rows this one is: the attempt
// before the call, then one of the three outcomes after it.
type ProduceKind string

const (
	ProduceKindAttempted ProduceKind = "attempted"
	ProduceKindCommitted ProduceKind = "committed"
	ProduceKindRejected  ProduceKind = "rejected"
	ProduceKindUnknown   ProduceKind = "unknown" // the reply was lost: the row may or may not exist
)

// ProduceRecord is one row of produce_record. Only committed rows carry a
// MessageId; only rejected rows carry a Code; rejected and unknown rows carry
// the Error text.
type ProduceRecord struct {
	At          time.Time   `json:"at"`
	Kind        ProduceKind `json:"kind"`
	Producer    string      `json:"producer"`
	Sequence    int64       `json:"sequence"`
	Key         string      `json:"key"`
	ScheduledAt time.Time   `json:"scheduled_at"`
	MessageId   int64       `json:"message_id"` // 0 unless committed
	Duplicate   bool        `json:"duplicate"`
	Code        string      `json:"code"`  // "" unless rejected
	Error       string      `json:"error"` // "" unless rejected or unknown
}
