package sqlstreamstest

import (
	"context"
	"log/slog"
	"sync"
)

// CountingLogger is a logging.Logger that counts records by level and by
// their code attribute, for asserting on log events without matching
// message text.
type CountingLogger struct {
	mutex  sync.Mutex
	counts map[countingLoggerKey]int
}

type countingLoggerKey struct {
	level slog.Level
	code  string
}

func NewCountingLogger() *CountingLogger {
	return &CountingLogger{counts: map[countingLoggerKey]int{}}
}

// Count returns how many records were logged at level with the given code
// attribute. An empty code counts every record at the level.
func (l *CountingLogger) Count(level slog.Level, code string) int {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if code == "" {
		total := 0
		for key, count := range l.counts {
			if key.level == level {
				total += count
			}
		}
		return total
	}
	return l.counts[countingLoggerKey{level: level, code: code}]
}

func (l *CountingLogger) DebugContext(ctx context.Context, message string, args ...any) {
	l.record(slog.LevelDebug, args)
}

func (l *CountingLogger) InfoContext(ctx context.Context, message string, args ...any) {
	l.record(slog.LevelInfo, args)
}

func (l *CountingLogger) WarnContext(ctx context.Context, message string, args ...any) {
	l.record(slog.LevelWarn, args)
}

func (l *CountingLogger) ErrorContext(ctx context.Context, message string, args ...any) {
	l.record(slog.LevelError, args)
}

func (l *CountingLogger) record(level slog.Level, args []any) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.counts[countingLoggerKey{level: level, code: codeAttribute(args)}]++
}

// ***************
// *** HELPERS ***
// ***************

// codeAttribute reads the "code" attribute out of a log call's key-value
// args, whether passed as a pair or as a slog.Attr; "" when absent.
func codeAttribute(args []any) string {
	for i := 0; i < len(args); i++ {
		switch value := args[i].(type) {
		case slog.Attr:
			if value.Key == "code" {
				return value.Value.String()
			}
		case string:
			if value == "code" && i+1 < len(args) {
				if code, ok := args[i+1].(string); ok {
					return code
				}
			}
			i++
		}
	}
	return ""
}
