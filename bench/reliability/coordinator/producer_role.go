package coordinator

import (
	"context"
	"fmt"
	"time"

	"github.com/agentstax/vulkan/bench/reliability/ledger"
	"github.com/agentstax/vulkan/bench/reliability/producer"
	"github.com/agentstax/vulkan/bench/reliability/scenario"
)

// inFlightLimit bounds the produces a stalled database can leave running at
// once; the pacer blocks past it, and the late scheduled_at shows the stall.
const inFlightLimit = 256

// RunProducer registers, then walks the producer phases in order, pacing
// the verifiable producer at each phase's rate. Returns when the last phase
// ends, or nil early when ctx is cancelled -- produces still in flight then
// land in the ledger as unknown.
func (c *Coordinator) RunProducer(ctx context.Context) error {
	orders, err := c.registerTopic(ctx)
	if err != nil {
		return err
	}
	instance, err := orders.Producer().Register(ctx, nil)
	if err != nil {
		return err
	}

	produces, err := c.openLedger(ledger.FileProduce)
	if err != nil {
		return err
	}
	defer produces.Close()
	phases, err := c.openLedger(ledger.FilePhase)
	if err != nil {
		return err
	}
	defer phases.Close()
	verifiable, err := producer.NewProducer(instance, produces, c.name)
	if err != nil {
		return err
	}

	for _, phase := range c.declared.Producer {
		if err := c.runProducerPhase(ctx, phase, verifiable, phases); err != nil {
			return ignoreCancellation(err)
		}
	}
	return nil
}

func (c *Coordinator) runProducerPhase(ctx context.Context, phase scenario.ProducerPhase, verifiable *producer.Producer, phases *ledger.Writer) error {
	pacer, err := NewPacer(phase.Rate, phase.Duration, inFlightLimit)
	if err != nil {
		return err
	}

	if err := c.writePhase(phases, ledger.PhaseProducer, phase.Name, ledger.PhaseStarted, phase.String()); err != nil {
		return err
	}
	runErr := pacer.Run(ctx, func(ctx context.Context, scheduled time.Time) {
		if err := verifiable.Produce(ctx, scheduled); err != nil {
			panic(fmt.Errorf("ledger write: %w", err))
		}
	})
	if err := c.writePhase(phases, ledger.PhaseProducer, phase.Name, ledger.PhaseEnded, phase.String()); err != nil {
		return err
	}
	return runErr
}
