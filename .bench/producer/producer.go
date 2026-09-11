package producer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/agentstax/sqlstreams/.bench/common"
	"github.com/agentstax/sqlstreams/.bench/record"
	sqlstreams "github.com/agentstax/sqlstreams/pkg/sqlstreams"
)

// Producer is the producer that writes a record per call: every Produce writes its attempt to
// the records, calls the library once, and writes what came back. It knows
// nothing of phases or rates; the runner decides when it is called.
type Producer struct {
	instance *sqlstreams.ProducerInstance[common.Order]
	writer   *record.Writer
	progress *record.Progress
	stream   string
	name     string
	sequence atomic.Int64
	Config   *ProducerConfig
}

// NewProducer is one stream's recording producer; keys restart at 1 per
// stream, so a key names a message only together with its stream.
func NewProducer(instance *sqlstreams.ProducerInstance[common.Order], writer *record.Writer, progress *record.Progress, stream string, name string, cfg *ProducerConfig) (*Producer, error) {
	if instance == nil {
		return nil, errors.New("instance must not be nil")
	}
	if writer == nil {
		return nil, errors.New("writer must not be nil")
	}
	if stream == "" {
		return nil, errors.New("stream must not be empty")
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
	if cfg.DisableMessageRecording && progress == nil {
		return nil, errors.New("progress is required when message recording is disabled")
	}
	return &Producer{instance: instance, writer: writer, progress: progress, stream: stream, name: name, Config: cfg}, nil
}

// Produce is one scheduled call: the attempt goes to the records first, then
// the outcome, so a process killed in between leaves the attempt on disk.
// A record write failing is a lab failure, never a produce outcome.
func (p *Producer) Produce(ctx context.Context, scheduled time.Time) error {
	order, err := p.newOrder()
	if err != nil {
		return err
	}
	var row record.ProduceRecord
	if p.Config.DisableMessageRecording {
		p.progress.Attempted.Add(1)
	} else {
		row = record.ProduceRecord{At: time.Now(), Kind: record.ProduceKindAttempted, Stream: p.stream, Producer: order.Producer, Sequence: order.Sequence, Key: order.Key(), ScheduledAt: scheduled}
		if err := p.writer.Write(row); err != nil {
			return err
		}
	}
	options := &sqlstreams.ProduceOptions{}
	if !p.Config.AutomaticBatching {
		options.IdempotencyKey = order.Key()
	}
	result, err := p.instance.Produce(ctx, order, options)
	if p.Config.DisableMessageRecording {
		p.countOutcome(1, err)
		return nil
	}
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

func (p *Producer) newOrder() (*common.Order, error) {
	order := &common.Order{Producer: p.name, Sequence: p.sequence.Add(1)}
	if p.Config.PayloadBytes > 0 {
		encoded, err := json.Marshal(order)
		if err != nil {
			return nil, err
		}
		padding := p.Config.PayloadBytes - len(encoded) - len(`,"padding":""`)
		if padding < 1 {
			return nil, fmt.Errorf("PayloadBytes must fit message identity, got %d", p.Config.PayloadBytes)
		}
		order.Padding = strings.Repeat("x", padding)
	}
	return order, nil
}

// ProduceBatch keeps one attempt and outcome per message, including ambiguous batch failures.
func (p *Producer) ProduceBatch(ctx context.Context, scheduled time.Time, size int) error {
	items := make([]*sqlstreams.ProduceItem[common.Order], size)
	var rows []record.ProduceRecord
	if !p.Config.DisableMessageRecording {
		rows = make([]record.ProduceRecord, size)
	}
	for i := range items {
		order, err := p.newOrder()
		if err != nil {
			return err
		}
		item, err := sqlstreams.NewProduceItem(order, nil)
		if err != nil {
			return err
		}
		items[i] = item
		if p.Config.DisableMessageRecording {
			continue
		}
		rows[i] = record.ProduceRecord{At: time.Now(), Kind: record.ProduceKindAttempted,
			Stream: p.stream, Producer: p.name, Sequence: order.Sequence,
			Key: order.Key(), ScheduledAt: scheduled}
		if err := p.writer.Write(rows[i]); err != nil {
			return err
		}
	}
	if p.Config.DisableMessageRecording {
		p.progress.Attempted.Add(int64(size))
	}
	results, err := p.instance.ProduceBatch(ctx, items...)
	completed := time.Now()
	if err == nil && len(results) != len(items) {
		return errors.New("batch result count differs from item count")
	}
	if p.Config.DisableMessageRecording {
		p.countOutcome(int64(size), err)
		return nil
	}
	for i := range rows {
		rows[i].At = completed
		if err == nil {
			rows[i].Kind = record.ProduceKindCommitted
			rows[i].MessageId = results[i].Id
			rows[i].Duplicate = results[i].Duplicate
		} else {
			rows[i].Kind, rows[i].Code = classify(err)
			rows[i].Error = err.Error()
		}
		if err := p.writer.Write(rows[i]); err != nil {
			return err
		}
	}
	return nil
}

func (p *Producer) countOutcome(count int64, err error) {
	if err == nil {
		p.progress.Committed.Add(count)
		return
	}
	kind, _ := classify(err)
	if kind == record.ProduceKindRejected {
		p.progress.Rejected.Add(count)
	} else {
		p.progress.Unknown.Add(count)
	}
}
