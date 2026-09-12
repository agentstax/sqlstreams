package record

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileKind names which table the checker COPYs a file into.
type FileKind string

const (
	FileKindContainer FileKind = "container"
	FileKindProgress  FileKind = "progress"
	FileKindProduce   FileKind = "produce"
	FileKindHandler   FileKind = "handler"
	FileKindPhase     FileKind = "phase"
	FileKindSample    FileKind = "sample"
	FileKindBacklog   FileKind = "backlog"
	FileKindStatement FileKind = "statement"
)

// Writer appends one JSON object per line to a role's record file and
// flushes every line, so a row is in the kernel before the call it records
// is made -- a SIGKILL in between leaves the attempt on disk.
type Writer struct {
	mutex    sync.Mutex
	file     *os.File
	buffered *bufio.Writer
	encoder  *json.Encoder
}

// NewWriter opens <dir>/<name>.<kind>.jsonl for append, creating dir.
func NewWriter(dir string, name string, kind FileKind) (*Writer, error) {
	if dir == "" {
		return nil, errors.New("dir must not be empty")
	}
	if name == "" {
		return nil, errors.New("name must not be empty")
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, fmt.Sprintf("%s.%s.jsonl", name, kind))
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	buffered := bufio.NewWriter(file)
	return &Writer{file: file, buffered: buffered, encoder: json.NewEncoder(buffered)}, nil
}

func (w *Writer) Write(row any) error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if err := w.encoder.Encode(row); err != nil {
		return err
	}
	return w.buffered.Flush()
}

func (w *Writer) Close() error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if err := w.buffered.Flush(); err != nil {
		return err
	}
	return w.file.Close()
}
