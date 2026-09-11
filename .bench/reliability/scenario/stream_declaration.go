package scenario

import (
	"errors"
	"fmt"
	"time"

	"github.com/agentstax/sqlstreams/pkg/consumer"
	"github.com/agentstax/sqlstreams/pkg/stream"
)

// StreamDeclaration is one stream under test and the consumer groups on it.
// Every stream runs the scenario's producer phases at the phase's rate.
type StreamDeclaration struct {
	Name              string
	DeliveryLogMode   stream.DeliveryLogMode
	PartitionSize     int64
	RetentionTTL      time.Duration
	IdempotencyKeyTTL time.Duration
	Janitor           *stream.JanitorConfig
	Vacuum            *stream.VacuumConfig
	VacuumEnabled     bool
	Groups            []GroupDeclaration
}

func (t StreamDeclaration) Validate() error {
	if t.Name == "" {
		return errors.New("Name is required")
	}

	if t.DeliveryLogMode != stream.DeliveryLogModeAll && t.DeliveryLogMode != stream.DeliveryLogModeFailures {
		return fmt.Errorf("DeliveryLogMode must be %q or %q, got %q", stream.DeliveryLogModeAll, stream.DeliveryLogModeFailures, t.DeliveryLogMode)
	}
	if len(t.Groups) == 0 {
		return errors.New("Groups must not be empty")
	}
	if err := (&stream.StreamConfig{PartitionSize: t.PartitionSize, DeliveryLogMode: t.DeliveryLogMode, RetentionTTL: t.RetentionTTL, IdempotencyKeyTTL: t.IdempotencyKeyTTL, Janitor: t.Janitor, Vacuum: t.Vacuum}).WithDefaults().Validate(); err != nil {
		return err
	}
	names := map[string]bool{}
	for i, group := range t.Groups {
		if err := group.Validate(); err != nil {
			return fmt.Errorf("Groups[%d]: %w", i, err)
		}
		if names[group.Name] {
			return fmt.Errorf("Groups[%d].Name already declared: %q", i, group.Name)
		}
		names[group.Name] = true
	}
	return nil
}

// GroupDeclaration is one consumer group: its name, the share of handler
// invocations the lab fails on purpose, the retries before a message is
// dead-lettered, and the messages each instance claims per poll -- 0 leaves
// the library's default, which a quiet run keeps and a ladder raises.
type GroupDeclaration struct {
	Name               string
	HandlerFailRate    float64
	MaxRetries         int
	BatchLimit         int
	QueueSize          int
	MessageConcurrency int
	ClaimPollRate      time.Duration
}

func (g GroupDeclaration) Validate() error {
	if g.Name == "" {
		return errors.New("Name is required")
	}
	if g.HandlerFailRate < 0 || g.HandlerFailRate > 1 {
		return fmt.Errorf("HandlerFailRate must be between 0 and 1, got %g", g.HandlerFailRate)
	}
	if g.MaxRetries < 0 {
		return fmt.Errorf("MaxRetries must be >= 0, got %d", g.MaxRetries)
	}
	if g.BatchLimit < 0 {
		return fmt.Errorf("BatchLimit must be >= 0, got %d", g.BatchLimit)
	}
	return (&consumer.ConsumeOptions{BatchLimit: g.BatchLimit, QueueSize: g.QueueSize, MessageConcurrency: g.MessageConcurrency, ClaimPollRate: g.ClaimPollRate}).WithDefaults().Validate()
}

// String is the [input] consumers line after the group's name.
func (g GroupDeclaration) String() string {
	line := fmt.Sprintf("handler fail rate %g, %d retries then dead", g.HandlerFailRate, g.MaxRetries)
	if g.BatchLimit > 0 {
		line += fmt.Sprintf(", batch %d", g.BatchLimit)
	}
	if g.QueueSize > 0 || g.MessageConcurrency > 0 || g.ClaimPollRate > 0 {
		line += fmt.Sprintf(", queue %d, concurrency %d, idle poll %s", g.QueueSize, g.MessageConcurrency, g.ClaimPollRate)
	}
	return line
}
