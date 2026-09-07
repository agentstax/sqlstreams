package checker

import (
	"bufio"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5"
)

// maxLineBytes bounds one record line; the longest field is an error text.
const maxLineBytes = 1 << 20

// lineSource feeds COPY one decoded JSON line at a time, so a file is never
// held in memory whole.
type lineSource struct {
	scanner *bufio.Scanner
	decode  func(line []byte) ([]any, error)
	line    int
	row     []any
	err     error
}

func newLineSource(reader io.Reader, decode func(line []byte) ([]any, error)) *lineSource {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)
	return &lineSource{scanner: scanner, decode: decode}
}

var _ pgx.CopyFromSource = (*lineSource)(nil)

func (s *lineSource) Next() bool {
	if s.err != nil || !s.scanner.Scan() {
		if err := s.scanner.Err(); err != nil && s.err == nil {
			s.err = err
		}
		return false
	}
	s.line++

	row, err := s.decode(s.scanner.Bytes())
	if err != nil {
		s.err = fmt.Errorf("line %d: %w", s.line, err)
		return false
	}
	s.row = row
	return true
}

func (s *lineSource) Values() ([]any, error) {
	return s.row, nil
}

func (s *lineSource) Err() error {
	return s.err
}
