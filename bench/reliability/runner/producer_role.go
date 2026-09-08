package runner

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/agentstax/vulkan/bench/reliability/producer"
	"github.com/agentstax/vulkan/bench/reliability/record"
	"github.com/agentstax/vulkan/bench/reliability/scenario"
)

// inFlightSeconds bounds the produces a stalled database can leave running
// at once per topic, as seconds of the phase's rate: past it the pacer
// blocks and the late scheduled_at shows the stall. Below inFlightFloor the
// bound is the floor, so a slow rate still rides out a short stall.
const (
	inFlightSeconds = 2
	inFlightFloor   = 256
)

// RunProducer registers every topic, then walks the producer phases on all
// of them at once, each topic paced at the phase's rate by its own recording
// producer. Returns when the last phase ends, or nil early when ctx is
// cancelled -- produces still in flight then land in the records as unknown.
func (r *Runner) RunProducer(ctx context.Context) error {
	topics, err := r.registerTopics(ctx)
	if err != nil {
		return err
	}

	produceRecords, err := r.openWriter(record.FileKindProduce)
	if err != nil {
		return err
	}
	defer produceRecords.Close()
	phaseRecords, err := r.openWriter(record.FileKindPhase)
	if err != nil {
		return err
	}
	defer phaseRecords.Close()

	producers := make([]*producer.Producer, 0, len(topics))
	for _, registered := range topics {
		instance, err := registered.handle.Producer().Register(ctx, producerConfig(r.declared))
		if err != nil {
			return err
		}
		recordingProducer, err := producer.NewProducer(instance, produceRecords, registered.declared.Name, r.name, &producer.ProducerConfig{AutomaticBatching: r.declared.AutomaticBatching, PayloadBytes: r.declared.PayloadBytes})
		if err != nil {
			return err
		}
		producers = append(producers, recordingProducer)
	}

	var routines errgroup.Group
	for i, registered := range topics {
		recordingProducer := producers[i]
		topicName := registered.declared.Name
		routines.Go(func() error {
			for _, phase := range r.declared.Producer {
				if err := r.runProducerPhase(ctx, phase, topicName, recordingProducer, phaseRecords); err != nil {
					return err
				}
			}
			return nil
		})
	}
	return ignoreCancellation(routines.Wait())
}

// runProducerPhase paces one topic through one phase; its phase rows carry
// the topic in Detail, and the checker takes the earliest start and latest
// end across topics as the phase's window.
func (r *Runner) runProducerPhase(ctx context.Context, phase scenario.ProducerPhase, topicName string, recordingProducer *producer.Producer, phaseRecords *record.Writer) error {
	pacer, err := NewPacer(phase.Rate, phase.Duration, max(inFlightFloor, phase.Rate*inFlightSeconds))
	if err != nil {
		return err
	}

	detail := topicName + ": " + phase.String()
	if err := r.writePhase(phaseRecords, record.PhaseKindProducer, phase.Name, record.PhaseStatusStarted, detail); err != nil {
		return err
	}
	runErr := pacer.Run(ctx, func(ctx context.Context, scheduled time.Time) {
		if err := recordingProducer.Produce(ctx, scheduled); err != nil {
			panic(fmt.Errorf("record write: %w", err))
		}
	})
	if err := r.writePhase(phaseRecords, record.PhaseKindProducer, phase.Name, record.PhaseStatusEnded, detail); err != nil {
		return err
	}
	return runErr
}
