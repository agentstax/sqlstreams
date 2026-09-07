package datastore

import "context"

// witnessLimit is how many example ids or keys a check keeps beside its
// count: enough to look up, few enough to print on one line.
const witnessLimit = 5

// witness labels: what a check's example values are
const (
	witnessKey       = "key"
	witnessMessageId = "message_id"
)

// Measurement is what every check query returns: how many rows matched and
// the first few, as text, so the report can name them -- Witness says
// whether they are keys or message ids.
type Measurement struct {
	Count     int64
	Witness   string
	Witnesses []string
}

// measure runs a check query shaped `SELECT count(*), <array of witnesses>`.
func (d *CheckerDatastore) measure(ctx context.Context, witness string, sql string, args ...any) (Measurement, error) {
	measured := Measurement{Witness: witness}
	err := d.pool.QueryRow(ctx, sql, args...).Scan(&measured.Count, &measured.Witnesses)
	return measured, err
}
