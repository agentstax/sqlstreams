package record

import "time"

// StatementRecord is one second of pg_stat_statements summed per statement
// shape: the server's normalized text with whitespace collapsed and every
// per-stream table suffix folded to _N, so one stream's statement and the
// next's count as one. pg_stat_statements keeps a statement from its first
// token, so the `-- sqlstreams:` owner comment never reaches it; the lab's
// own reads carry an inline marker and sum under "lab". Cumulative since
// the server started, so the checker takes the difference between samples.
type StatementRecord struct {
	At     time.Time `json:"at"`
	Query  string    `json:"query"`
	Calls  int64     `json:"calls"`
	ExecMs float64   `json:"exec_ms"`
	Rows   int64     `json:"rows"`
}
