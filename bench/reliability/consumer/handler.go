package consumer

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/agentstax/vulkan/bench/reliability/lab"
	"github.com/agentstax/vulkan/bench/reliability/ledger"
	vulkan "github.com/agentstax/vulkan/pkg/vulkan"
)

// errInjectedFailure is the handler's own failure under the scenario's fail
// rate; the delivery goes through the library's retry path like any other
// handler error.
var errInjectedFailure = errors.New("handler failure injected by the scenario's fail rate")

// Handler is one consumer instance's handler: it writes one ledger fact per
// invocation before returning, and fails the scenario's share of them.
type Handler struct {
	consumer string
	group    string
	failRate float64
	handled  *ledger.Writer
}

func NewHandler(consumer string, group string, failRate float64, handled *ledger.Writer) (*Handler, error) {
	if consumer == "" {
		return nil, errors.New("consumer must not be empty")
	}
	if group == "" {
		return nil, errors.New("group must not be empty")
	}
	if failRate < 0 || failRate > 1 {
		return nil, fmt.Errorf("failRate must be between 0 and 1, got %g", failRate)
	}
	if handled == nil {
		return nil, errors.New("handled must not be nil")
	}
	return &Handler{consumer: consumer, group: group, failRate: failRate, handled: handled}, nil
}

func (h *Handler) Handle(ctx context.Context, order *lab.Order) error {
	meta, ok := vulkan.MetaFromContext(ctx)
	if !ok {
		return errors.New("message meta is missing from the handler ctx")
	}

	outcome := ledger.HandlerSuccess
	if h.failRate > 0 && rand.Float64() < h.failRate {
		outcome = ledger.HandlerError
	}
	fact := ledger.HandlerFact{
		At:        time.Now(),
		Consumer:  h.consumer,
		Group:     h.group,
		MessageId: meta.Id,
		Key:       fmt.Sprintf("%s-%d", order.Producer, order.Seq),
		Attempt:   meta.Attempts,
		Outcome:   outcome,
	}
	if err := h.handled.Write(fact); err != nil {
		return err
	}
	if outcome == ledger.HandlerError {
		return errInjectedFailure
	}
	return nil
}
