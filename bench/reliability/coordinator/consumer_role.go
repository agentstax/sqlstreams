package coordinator

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentstax/vulkan/bench/reliability/consumer"
	"github.com/agentstax/vulkan/bench/reliability/lab"
	"github.com/agentstax/vulkan/bench/reliability/ledger"
	"github.com/agentstax/vulkan/bench/reliability/scenario"
)

// RunConsumer registers, applies each consumer change at its offset, then
// holds the last count until ctx is cancelled -- the checker decides when
// the topic has drained, not the consumer. Every instance is stopped before
// returning. A Consume session failing on its own ends the run with its
// error.
func (c *Coordinator) RunConsumer(ctx context.Context) error {
	orders, err := c.registerTopic(ctx)
	if err != nil {
		return err
	}

	handled, err := c.openLedger(ledger.FileHandler)
	if err != nil {
		return err
	}
	defer handled.Close()
	phases, err := c.openLedger(ledger.FilePhase)
	if err != nil {
		return err
	}
	defer phases.Close()
	fleet, err := consumer.NewInstances(orders.Consumer(c.declared.Group), c.consumerConfig(), c.declared.Group, c.declared.HandlerFailRate, handled, c.name)
	if err != nil {
		return err
	}

	start := time.Now()
	for _, change := range c.declared.Consumers {
		if err := lab.WaitUntil(ctx, start.Add(change.At)); err != nil {
			return stopFleet(fleet, ignoreCancellation(err))
		}
		if err := fleet.SetCount(ctx, change.Instances); err != nil {
			return stopFleet(fleet, err)
		}
		if err := c.writeConsumerPhase(phases, change); err != nil {
			return stopFleet(fleet, err)
		}
	}

	select {
	case <-ctx.Done():
		return stopFleet(fleet, nil)
	case err := <-fleet.Failed():
		return stopFleet(fleet, err)
	}
}

func (c *Coordinator) writeConsumerPhase(phases *ledger.Writer, change scenario.ConsumerChange) error {
	name := fmt.Sprintf("consumers %d", change.Instances)
	detail := strings.Join(strings.Fields(change.String()), " ")
	return c.writePhase(phases, ledger.PhaseConsumers, name, ledger.PhaseStarted, detail)
}

// ***************
// *** HELPERS ***
// ***************

// stopFleet stops every instance and returns cause; the stop must outlive
// the cancelled run ctx so each session can return.
func stopFleet(fleet *consumer.Instances, cause error) error {
	if err := fleet.SetCount(context.WithoutCancel(context.Background()), 0); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}
