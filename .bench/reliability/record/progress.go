package record

import (
	"errors"
	"sync/atomic"
	"time"
)

// Progress holds cumulative message counters for one process and stream/group.
// It records counts, not message identities or crash-recoverable outcomes.
type Progress struct {
	writer    *Writer
	process   string
	stream    string
	group     string
	Attempted atomic.Int64
	Committed atomic.Int64
	Rejected  atomic.Int64
	Unknown   atomic.Int64
	Success   atomic.Int64
	Error     atomic.Int64
}

func NewProgress(writer *Writer, process string, stream string, group string) (*Progress, error) {
	if writer == nil {
		return nil, errors.New("writer must not be nil")
	}
	if process == "" || stream == "" {
		return nil, errors.New("process and stream are required")
	}
	return &Progress{writer: writer, process: process, stream: stream, group: group}, nil
}

// Snapshot writes the current counters; concurrent completions can straddle a snapshot.
func (p *Progress) Snapshot() error {
	return p.writer.Write(ProgressRecord{At: time.Now(), Process: p.process, Stream: p.stream, Group: p.group,
		Attempted: p.Attempted.Load(), Committed: p.Committed.Load(), Rejected: p.Rejected.Load(), Unknown: p.Unknown.Load(), Success: p.Success.Load(), Error: p.Error.Load()})
}

// ProgressRecord is a cumulative snapshot. Group is empty for producers.
// Loaded snapshots bound measurements; later completions are outside that sample.
type ProgressRecord struct {
	At        time.Time `json:"at"`
	Process   string    `json:"process"`
	Stream    string    `json:"stream"`
	Group     string    `json:"group"`
	Attempted int64     `json:"attempted"`
	Committed int64     `json:"committed"`
	Rejected  int64     `json:"rejected"`
	Unknown   int64     `json:"unknown"`
	Success   int64     `json:"success"`
	Error     int64     `json:"error"`
}
