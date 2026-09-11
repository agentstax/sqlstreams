package consumer

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/agentstax/sqlstreams/.bench/common"
	"github.com/agentstax/sqlstreams/.bench/record"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
)

// errInjectedFailure is the handler's own failure under the scenario's fail
// rate; the delivery goes through the library's retry path like any other
// handler error.
var errInjectedFailure = errors.New("handler failure injected by the scenario's fail rate")

// Handler is one consumer instance's handler: it writes one record per
// invocation before returning, and fails the scenario's share of them.
type Handler struct {
	name     string
	stream   string
	group    string
	failRate float64
	writer   *record.HandlerWriter
	progress *record.Progress
	Config   *HandlerConfig
	failed   chan error
}

func NewHandler(name string, stream string, group string, failRate float64, writer *record.HandlerWriter, progress *record.Progress, failed chan error, cfg *HandlerConfig) (*Handler, error) {
	if name == "" {
		return nil, errors.New("name must not be empty")
	}
	if stream == "" {
		return nil, errors.New("stream must not be empty")
	}
	if group == "" {
		return nil, errors.New("group must not be empty")
	}
	if failRate < 0 || failRate > 1 {
		return nil, fmt.Errorf("failRate must be between 0 and 1, got %g", failRate)
	}
	if writer == nil {
		return nil, errors.New("writer must not be nil")
	}
	if cfg == nil {
		cfg = &HandlerConfig{}
	}
	cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.DisableMessageRecording && progress == nil {
		return nil, errors.New("progress is required when message recording is disabled")
	}
	if failed == nil {
		return nil, errors.New("failed must not be nil")
	}
	return &Handler{name: name, stream: stream, group: group, failRate: failRate, writer: writer, progress: progress, Config: cfg, failed: failed}, nil
}

// Handle writes the invocation's record, then returns the injected outcome.
// A record write failing is a lab failure, never a delivery outcome: it is
// sent on failed for the runner to stop on, and returned so the library does
// not record a success the records lack.
func (h *Handler) Handle(ctx context.Context, order *common.Order) error {
	meta, ok := sqlstreams.MetaFromContext(ctx)
	if !ok {
		return errors.New("message meta is missing from the handler ctx")
	}

	outcome := record.HandlerOutcomeSuccess
	if h.failRate > 0 && rand.Float64() < h.failRate {
		outcome = record.HandlerOutcomeError
	}
	if h.Config.DisableMessageRecording {
		if outcome == record.HandlerOutcomeError {
			h.progress.Error.Add(1)
			return errInjectedFailure
		}
		h.progress.Success.Add(1)
		return nil
	}
	row := record.HandlerRecord{
		At:        time.Now(),
		Consumer:  h.name,
		Stream:    h.stream,
		Group:     h.group,
		MessageId: meta.Id,
		Key:       order.Key(),
		Attempt:   meta.Attempts,
		Outcome:   outcome,
	}
	if err := h.writer.Write(row); err != nil {
		select {
		case h.failed <- fmt.Errorf("%s: record write: %w", h.name, err):
		default:
		}
		return err
	}
	if outcome == record.HandlerOutcomeError {
		return errInjectedFailure
	}
	return nil
}
