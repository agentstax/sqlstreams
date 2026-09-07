package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/agentstax/vulkan/bench/reliability/producer"
	"github.com/agentstax/vulkan/bench/reliability/record"
	"github.com/agentstax/vulkan/bench/reliability/scenario"
)

// inFlightLimit bounds the produces a stalled database can leave running at
// once; the pacer blocks past it, and the late scheduled_at shows the stall.
const inFlightLimit = 256

// RunProducer registers, then walks the producer phases in order, pacing
// the verifiable producer at each phase's rate. Returns when the last phase
// ends, or nil early when ctx is cancelled -- produces still in flight then
// land in the records as unknown.
func (r *Runner) RunProducer(ctx context.Context) error {
	orders, err := r.registerTopic(ctx)
	if err != nil {
		return err
	}
	instance, err := orders.Producer().Register(ctx, nil)
	if err != nil {
		return err
	}

	produceRecords, err := r.openWriter(record.FileProduce)
	if err != nil {
		return err
	}
	defer produceRecords.Close()
	phaseRecords, err := r.openWriter(record.FilePhase)
	if err != nil {
		return err
	}
	defer phaseRecords.Close()
	verifiable, err := producer.NewProducer(instance, produceRecords, r.name)
	if err != nil {
		return err
	}

	for _, phase := range r.declared.Producer {
		if err := r.runProducerPhase(ctx, phase, verifiable, phaseRecords); err != nil {
			return ignoreCancellation(err)
		}
	}
	return nil
}

func (r *Runner) runProducerPhase(ctx context.Context, phase scenario.ProducerPhase, verifiable *producer.Producer, phaseRecords *record.Writer) error {
	pacer, err := NewPacer(phase.Rate, phase.Duration, inFlightLimit)
	if err != nil {
		return err
	}

	if err := r.writePhase(phaseRecords, record.PhaseProducer, phase.Name, record.PhaseStarted, phase.String()); err != nil {
		return err
	}
	runErr := pacer.Run(ctx, func(ctx context.Context, scheduled time.Time) {
		if err := verifiable.Produce(ctx, scheduled); err != nil {
			panic(fmt.Errorf("record write: %w", err))
		}
	})
	if err := r.writePhase(phaseRecords, record.PhaseProducer, phase.Name, record.PhaseEnded, phase.String()); err != nil {
		return err
	}
	return runErr
}
