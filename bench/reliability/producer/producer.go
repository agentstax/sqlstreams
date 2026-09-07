package producer

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/agentstax/vulkan/bench/reliability/lab"
	"github.com/agentstax/vulkan/bench/reliability/ledger"
	vulkan "github.com/agentstax/vulkan/pkg/vulkan"
)

// Producer is the verifiable producer: every Produce writes its attempt to
// the ledger, calls the library once, and writes what came back. It knows
// nothing of phases or rates; the coordinator decides when it is called.
type Producer struct {
	instance *vulkan.ProducerInstance[lab.Order]
	produces *ledger.Writer
	name     string
	seq      atomic.Int64
}

func NewProducer(instance *vulkan.ProducerInstance[lab.Order], produces *ledger.Writer, name string) (*Producer, error) {
	if instance == nil {
		return nil, errors.New("instance must not be nil")
	}
	if produces == nil {
		return nil, errors.New("produces must not be nil")
	}
	if name == "" {
		return nil, errors.New("name must not be empty")
	}
	return &Producer{instance: instance, produces: produces, name: name}, nil
}

// Produce is one scheduled call: the attempt goes to the ledger first, then
// the outcome, so a process killed in between leaves the attempt on disk.
// A ledger write failing is a lab failure, never a produce outcome.
func (p *Producer) Produce(ctx context.Context, scheduled time.Time) error {
	order := &lab.Order{Producer: p.name, Seq: p.seq.Add(1)}
	fact := ledger.ProduceFact{
		At:          time.Now(),
		Kind:        ledger.ProduceAttempted,
		Producer:    order.Producer,
		Seq:         order.Seq,
		Key:         order.Key(),
		ScheduledAt: scheduled,
	}
	if err := p.produces.Write(fact); err != nil {
		return err
	}

	result, err := p.instance.Produce(ctx, order, &vulkan.ProduceOptions{IdempotencyKey: order.Key()})
	fact.At = time.Now()
	if err == nil {
		fact.Kind = ledger.ProduceCommitted
		fact.MessageId = result.Id
		fact.Duplicate = result.Duplicate
	} else {
		fact.Kind, fact.Code = classify(err)
		fact.Error = err.Error()
	}
	return p.produces.Write(fact)
}
