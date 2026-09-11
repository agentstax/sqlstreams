package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentstax/sqlstreams/.bench/common"
	"github.com/agentstax/sqlstreams/.bench/consumer"
	"github.com/agentstax/sqlstreams/.bench/record"
	"github.com/agentstax/sqlstreams/.bench/scenario"
)

// RunConsumer registers every stream, applies each consumer change at its
// offset to every group of every stream, then holds the last count until ctx
// is cancelled -- the checker decides when the streams have drained, not the
// consumer. Every instance is stopped before returning. A Consume session
// failing on its own ends the run with its error.
func (r *Runner) RunConsumer(ctx context.Context) error {
	streams, err := r.registerStreams(ctx)
	if err != nil {
		return err
	}

	var instances int
	for _, change := range r.declared.Consumers {
		instances = max(instances, change.Instances)
	}
	var concurrency int
	for _, registered := range streams {
		for _, group := range registered.declared.Groups {
			concurrency += instances * consumeOptions(group).WithDefaults().MessageConcurrency
		}
	}
	handlerRecords, err := record.NewHandlerWriter(r.recordDir, r.name, max(1, concurrency))
	if err != nil {
		return err
	}
	defer handlerRecords.Close()
	phaseRecords, err := r.openWriter(record.FileKindPhase)
	if err != nil {
		return err
	}
	defer phaseRecords.Close()

	progressRecords, err := r.openWriter(record.FileKindProgress)
	if err != nil {
		return err
	}
	defer progressRecords.Close()
	progress := []*record.Progress{}
	failed := make(chan error, 1)
	groups := []*consumer.Instances{}
	for _, registered := range streams {
		for _, group := range registered.declared.Groups {
			var counters *record.Progress
			if r.declared.DisableMessageRecording {
				counters, err = record.NewProgress(progressRecords, r.name, registered.declared.Name, group.Name)
				if err != nil {
					return err
				}
				progress = append(progress, counters)
			}
			instances, err := consumer.NewInstances(registered.handle.Consumer(group.Name), consumerConfig(group), consumeOptions(group),
				registered.declared.Name, group.Name, group.HandlerFailRate, handlerRecords, counters, r.name, failed, &consumer.HandlerConfig{DisableMessageRecording: r.declared.DisableMessageRecording})
			if err != nil {
				return err
			}
			groups = append(groups, instances)
		}
	}

	if len(progress) == 0 {
		return r.runConsumerChanges(ctx, groups, phaseRecords, failed)
	}
	progressCtx, stopProgress := context.WithCancel(context.WithoutCancel(ctx))
	progressDone := make(chan error, 1)
	go func() {
		err := runProgress(progressCtx, progress)
		if err != nil {
			select {
			case failed <- err:
			default:
			}
		}
		progressDone <- err
	}()
	err = r.runConsumerChanges(ctx, groups, phaseRecords, failed)
	stopProgress()
	return errors.Join(err, <-progressDone)
}

func (r *Runner) runConsumerChanges(ctx context.Context, groups []*consumer.Instances, phaseRecords *record.Writer, failed <-chan error) error {
	start := time.Now()
	for _, change := range r.declared.Consumers {
		if err := common.WaitUntil(ctx, start.Add(change.At)); err != nil {
			return stopInstances(groups, ignoreCancellation(err))
		}
		for _, instances := range groups {
			if err := instances.SetCount(ctx, change.Instances); err != nil {
				return stopInstances(groups, err)
			}
		}
		if err := r.writeConsumerPhase(phaseRecords, change); err != nil {
			return stopInstances(groups, err)
		}
	}

	select {
	case <-ctx.Done():
		return stopInstances(groups, nil)
	case err := <-failed:
		return stopInstances(groups, err)
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

// stopInstances stops every group's instances and returns cause; the stop
// must outlive the cancelled run ctx so each session can return.
func stopInstances(groups []*consumer.Instances, cause error) error {
	for _, instances := range groups {
		if err := instances.SetCount(context.WithoutCancel(context.Background()), 0); err != nil {
			cause = errors.Join(cause, err)
		}
	}
	return cause
}
