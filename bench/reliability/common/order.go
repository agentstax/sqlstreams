package common

import "fmt"

// Order is the one message type the lab produces. Producer and Sequence are the
// two halves of the idempotency key, so a delivered payload names its own
// record row.
type Order struct {
	Producer string `json:"producer"`
	Sequence int64  `json:"sequence"`
}

// Value receivers: the library reads SchemaVersion off the zero value.
func (Order) SchemaVersion() int { return 1 }

// Key is the order's record key and idempotency key, "<producer>-<sequence>". The
// checker rebuilds it in SQL from the stored payload.
func (o Order) Key() string {
	return fmt.Sprintf("%s-%d", o.Producer, o.Sequence)
}
