package lab

// Order is the one message type the lab produces. Producer and Seq are the
// two halves of the idempotency key, so a delivered payload names its own
// ledger row.
type Order struct {
	Producer string `json:"producer"`
	Seq      int64  `json:"seq"`
}

func (Order) SchemaVersion() int { return 1 }
