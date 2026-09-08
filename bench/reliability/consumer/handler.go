package consumer

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/agentstax/vulkan/bench/reliability/common"
	"github.com/agentstax/vulkan/bench/reliability/record"
	vulkan "github.com/agentstax/vulkan/pkg/vulkan"
)

// errInjectedFailure is the handler's own failure under the scenario's fail
// rate; the delivery goes through the library's retry path like any other
// handler error.
var errInjectedFailure = errors.New("handler failure injected by the scenario's fail rate")

// Handler is one consumer instance's handler: it writes one record per
// invocation before returning, and fails the scenario's share of them.
type Handler struct {
	name     string
	topic    string
	group    string
	failRate float64
	writer   *record.Writer
	failed   chan error
}

func NewHandler(name string, topic string, group string, failRate float64, writer *record.Writer, failed chan error) (*Handler, error) {
	if name == "" {
		return nil, errors.New("name must not be empty")
	}
	if topic == "" {
		return nil, errors.New("topic must not be empty")
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
	if failed == nil {
		return nil, errors.New("failed must not be nil")
	}
	return &Handler{name: name, topic: topic, group: group, failRate: failRate, writer: writer, failed: failed}, nil
}

// Handle writes the invocation's record, then returns the injected outcome.
// A record write failing is a lab failure, never a delivery outcome: it is
// sent on failed for the runner to stop on, and returned so the library does
// not record a success the records lack.
func (h *Handler) Handle(ctx context.Context, order *common.Order) error {
	meta, ok := vulkan.MetaFromContext(ctx)
	if !ok {
		return errors.New("message meta is missing from the handler ctx")
	}

	outcome := record.HandlerOutcomeSuccess
	if h.failRate > 0 && rand.Float64() < h.failRate {
		outcome = record.HandlerOutcomeError
	}
	row := record.HandlerRecord{
		At:        time.Now(),
		Consumer:  h.name,
		Topic:     h.topic,
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
