package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentstax/vulkan/bench/reliability/common"
	"github.com/agentstax/vulkan/bench/reliability/consumer"
	"github.com/agentstax/vulkan/bench/reliability/record"
	"github.com/agentstax/vulkan/bench/reliability/scenario"
)

// RunConsumer registers, applies each consumer change at its offset, then
// holds the last count until ctx is cancelled -- the checker decides when
// the topic has drained, not the consumer. Every instance is stopped before
// returning. A Consume session failing on its own ends the run with its
// error.
func (r *Runner) RunConsumer(ctx context.Context) error {
	orders, err := r.registerTopic(ctx)
	if err != nil {
		return err
	}

	handlerRecords, err := r.openWriter(record.FileKindHandler)
	if err != nil {
		return err
	}
	defer handlerRecords.Close()
	phaseRecords, err := r.openWriter(record.FileKindPhase)
	if err != nil {
		return err
	}
	defer phaseRecords.Close()
	instances, err := consumer.NewInstances(orders.Consumer(r.declared.Group), r.consumerConfig(), r.declared.Group, r.declared.HandlerFailRate, handlerRecords, r.name)
	if err != nil {
		return err
	}

	start := time.Now()
	for _, change := range r.declared.Consumers {
		if err := common.WaitUntil(ctx, start.Add(change.At)); err != nil {
			return stopInstances(instances, ignoreCancellation(err))
		}
		if err := instances.SetCount(ctx, change.Instances); err != nil {
			return stopInstances(instances, err)
		}
		if err := r.writeConsumerPhase(phaseRecords, change); err != nil {
			return stopInstances(instances, err)
		}
	}

	select {
	case <-ctx.Done():
		return stopInstances(instances, nil)
	case err := <-instances.Failed():
		return stopInstances(instances, err)
	}
}

func (r *Runner) writeConsumerPhase(phaseRecords *record.Writer, change scenario.ConsumerChange) error {
	name := fmt.Sprintf("consumers %d", change.Instances)
	detail := strings.Join(strings.Fields(change.String()), " ")
	return r.writePhase(phaseRecords, record.PhaseKindConsumers, name, record.PhaseStatusStarted, detail)
}

// ***************
// *** HELPERS ***
// ***************

// stopInstances stops every instance and returns cause; the stop must
// outlive the cancelled run ctx so each session can return.
func stopInstances(instances *consumer.Instances, cause error) error {
	if err := instances.SetCount(context.WithoutCancel(context.Background()), 0); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}
