package producer

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/agentstax/vulkan/bench/reliability/common"
	"github.com/agentstax/vulkan/bench/reliability/record"
	vulkan "github.com/agentstax/vulkan/pkg/vulkan"
)

// Producer is the verifiable producer: every Produce writes its attempt to
// the records, calls the library once, and writes what came back. It knows
// nothing of phases or rates; the runner decides when it is called.
type Producer struct {
	instance *vulkan.ProducerInstance[common.Order]
	produces *record.Writer
	name     string
	seq      atomic.Int64
}

func NewProducer(instance *vulkan.ProducerInstance[common.Order], produces *record.Writer, name string) (*Producer, error) {
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

// Produce is one scheduled call: the attempt goes to the records first, then
// the outcome, so a process killed in between leaves the attempt on disk.
// A record write failing is a lab failure, never a produce outcome.
func (p *Producer) Produce(ctx context.Context, scheduled time.Time) error {
	order := &common.Order{Producer: p.name, Seq: p.seq.Add(1)}
	fact := record.Produce{
		At:          time.Now(),
		Kind:        record.ProduceAttempted,
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
		fact.Kind = record.ProduceCommitted
		fact.MessageId = result.Id
		fact.Duplicate = result.Duplicate
	} else {
		fact.Kind, fact.Code = classify(err)
		fact.Error = err.Error()
	}
	return p.produces.Write(fact)
}
