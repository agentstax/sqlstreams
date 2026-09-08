package producer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/agentstax/vulkan/bench/reliability/common"
	"github.com/agentstax/vulkan/bench/reliability/record"
	vulkan "github.com/agentstax/vulkan/pkg/vulkan"
)

// Producer is the producer that writes a record per call: every Produce writes its attempt to
// the records, calls the library once, and writes what came back. It knows
// nothing of phases or rates; the runner decides when it is called.
type Producer struct {
	instance *vulkan.ProducerInstance[common.Order]
	writer   *record.Writer
	topic    string
	name     string
	sequence atomic.Int64
	Config   *ProducerConfig
}

// NewProducer is one topic's recording producer; keys restart at 1 per
// topic, so a key names a message only together with its topic.
func NewProducer(instance *vulkan.ProducerInstance[common.Order], writer *record.Writer, topic string, name string, cfg *ProducerConfig) (*Producer, error) {
	if instance == nil {
		return nil, errors.New("instance must not be nil")
	}
	if writer == nil {
		return nil, errors.New("writer must not be nil")
	}
	if topic == "" {
		return nil, errors.New("topic must not be empty")
	}
	if name == "" {
		return nil, errors.New("name must not be empty")
	}
	if cfg == nil {
		cfg = &ProducerConfig{}
	}
	cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Producer{instance: instance, writer: writer, topic: topic, name: name, Config: cfg}, nil
}

// Produce is one scheduled call: the attempt goes to the records first, then
// the outcome, so a process killed in between leaves the attempt on disk.
// A record write failing is a lab failure, never a produce outcome.
func (p *Producer) Produce(ctx context.Context, scheduled time.Time) error {
	order := &common.Order{Producer: p.name, Sequence: p.sequence.Add(1)}
	if p.Config.PayloadBytes > 0 {
		encoded, err := json.Marshal(order)
		if err != nil {
			return err
		}
		padding := p.Config.PayloadBytes - len(encoded) - len(`,"padding":""`)
		if padding < 1 {
			return fmt.Errorf("PayloadBytes must fit message identity, got %d", p.Config.PayloadBytes)
		}
		order.Padding = strings.Repeat("x", padding)
	}
	row := record.ProduceRecord{
		At:          time.Now(),
		Kind:        record.ProduceKindAttempted,
		Topic:       p.topic,
		Producer:    order.Producer,
		Sequence:    order.Sequence,
		Key:         order.Key(),
		ScheduledAt: scheduled,
	}
	if err := p.writer.Write(row); err != nil {
		return err
	}

	options := &vulkan.ProduceOptions{}
	if !p.Config.AutomaticBatching {
		options.IdempotencyKey = order.Key()
	}
	result, err := p.instance.Produce(ctx, order, options)
	row.At = time.Now()
	if err == nil {
		row.Kind = record.ProduceKindCommitted
		row.MessageId = result.Id
		row.Duplicate = result.Duplicate
	} else {
		row.Kind, row.Code = classify(err)
		row.Error = err.Error()
	}
	return p.writer.Write(row)
}
