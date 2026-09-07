package record

import "time"

// ProduceKind is which of a produce's two facts a row records: the attempt
// before the call, then one of the three outcomes after it.
type ProduceKind string

const (
	ProduceAttempted ProduceKind = "attempted"
	ProduceCommitted ProduceKind = "committed"
	ProduceRejected  ProduceKind = "rejected"
	ProduceUnknown   ProduceKind = "unknown" // the reply was lost: the row may or may not exist
)

// Produce is one row of produce_ledger. Only committed rows carry a
// MessageId; only rejected rows carry a Code; rejected and unknown rows carry
// the Error text.
type Produce struct {
	At          time.Time   `json:"at"`
	Kind        ProduceKind `json:"kind"`
	Producer    string      `json:"producer"`
	Seq         int64       `json:"seq"`
	Key         string      `json:"key"`
	ScheduledAt time.Time   `json:"scheduled_at"`
	MessageId   int64       `json:"message_id"` // 0 unless committed
	Duplicate   bool        `json:"duplicate"`
	Code        string      `json:"code"`  // "" unless rejected
	Error       string      `json:"error"` // "" unless rejected or unknown
}
