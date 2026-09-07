package record

import "time"

// SampleRecord is one second of the server's own counters as the observer
// read them: cumulative since the last stats reset, so the checker takes
// the difference between two samples.
type SampleRecord struct {
	At          time.Time `json:"at"`
	WalRecords  int64     `json:"wal_records"`
	WalFpi      int64     `json:"wal_fpi"`
	WalBytes    int64     `json:"wal_bytes"`
	Checkpoints int64     `json:"checkpoints"`
	XactCommit  int64     `json:"xact_commit"`
	Deadlocks   int64     `json:"deadlocks"`
	BlocksHit   int64     `json:"blocks_hit"`
	BlocksRead  int64     `json:"blocks_read"`
}
