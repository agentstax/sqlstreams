package runner

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/agentstax/sqlstreams/.bench/reliability/producer"
	"github.com/agentstax/sqlstreams/.bench/reliability/record"
	"github.com/agentstax/sqlstreams/.bench/reliability/scenario"
)

// inFlightSeconds bounds the produces a stalled database can leave running
// at once per stream, as seconds of the phase's rate: past it the pacer
// blocks and the late scheduled_at shows the stall. Below inFlightFloor the
// bound is the floor, so a slow rate still rides out a short stall.
const (
	inFlightSeconds = 2
	inFlightFloor   = 256
)

// RunProducer registers every stream, then walks the producer phases on all
// of them at once, paced or with bounded unpaced callers as declared. Returns when the last phase ends, or nil early when ctx is
// cancelled -- produces still in flight then land in the records as unknown.
func (r *Runner) RunProducer(ctx context.Context) error {
	streams, err := r.registerStreams(ctx)
	if err != nil {
		return err
	}

	if r.declared.DisableExceptionConsumers {
		startupCtx, stop := context.WithTimeout(ctx, time.Minute)
		defer stop()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			if err := r.ds.SuspendExceptionConsumers(startupCtx); err != nil {
				return err
			}
			stopped, err := r.ds.ExceptionConsumersStopped(startupCtx)
			if err != nil {
				return err
			}
			if stopped {
				break
			}
			select {
			case <-startupCtx.Done():
				return startupCtx.Err()
			case <-ticker.C:
			}
		}
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

	progressRecords, err := r.openWriter(record.FileKindProgress)
	if err != nil {
		return err
	}
	defer progressRecords.Close()
	progress := []*record.Progress{}
	producers := make([]*producer.Producer, 0, len(streams))
	for _, registered := range streams {
		instance, err := registered.handle.Producer().Register(ctx, producerConfig(r.declared))
		if err != nil {
			return err
		}
		var counters *record.Progress
		if r.declared.DisableMessageRecording {
			counters, err = record.NewProgress(progressRecords, r.name, registered.declared.Name, "")
			if err != nil {
				return err
			}
			progress = append(progress, counters)
		}
		recordingProducer, err := producer.NewProducer(instance, produceRecords, counters, registered.declared.Name, r.name, &producer.ProducerConfig{DisableMessageRecording: r.declared.DisableMessageRecording, AutomaticBatching: r.declared.AutomaticBatching, PayloadBytes: r.declared.PayloadBytes})
		if err != nil {
			return err
		}
		producers = append(producers, recordingProducer)
	}

	running, runningCtx := errgroup.WithContext(ctx)
	progressCtx, stopProgress := context.WithCancel(context.WithoutCancel(runningCtx))
	defer stopProgress()
	if len(progress) > 0 {
		running.Go(func() error { return runProgress(progressCtx, progress) })
	}
	var routines errgroup.Group
	for i, registered := range streams {
		recordingProducer := producers[i]
		streamName := registered.declared.Name
		routines.Go(func() error {
			for _, phase := range r.declared.Producer {
				if err := r.runProducerPhase(runningCtx, phase, streamName, recordingProducer, phaseRecords); err != nil {
					return err
				}
			}
			return nil
		})
	}
	running.Go(func() error { defer stopProgress(); return routines.Wait() })
	return ignoreCancellation(running.Wait())
}

// runProducerPhase paces one stream through one phase; its phase rows carry
// the stream in Detail, and the checker takes the earliest start and latest
// end across streams as the phase's window.
func (r *Runner) runProducerPhase(ctx context.Context, phase scenario.ProducerPhase, streamName string, recordingProducer *producer.Producer, phaseRecords *record.Writer) error {
	if phase.Unpaced {
		if err := r.writePhase(phaseRecords, record.PhaseKindProducer, phase.Name, record.PhaseStatusStarted, streamName+": "+phase.String()); err != nil {
			return err
		}
		end := time.Now().Add(phase.Duration)
		routines, routinesCtx := errgroup.WithContext(ctx)
		for range r.declared.ProducerConcurrency {
			routines.Go(func() error {
				for time.Now().Before(end) {
					if err := routinesCtx.Err(); err != nil {
						return err
					}
					if r.declared.ExplicitBatching {
						if err := recordingProducer.ProduceBatch(routinesCtx, time.Now(), r.declared.ProducerBatchSize); err != nil {
							return err
						}
					} else if err := recordingProducer.Produce(routinesCtx, time.Now()); err != nil {
						return err
					}
				}
				return nil
			})
		}
		if err := routines.Wait(); err != nil {
			return err
		}
		return r.writePhase(phaseRecords, record.PhaseKindProducer, phase.Name, record.PhaseStatusEnded, streamName+": "+phase.String())
	}
	pacer, err := NewPacer(phase.Rate, phase.Duration, max(inFlightFloor, phase.Rate*inFlightSeconds))
	if err != nil {
		return err
	}

	detail := streamName + ": " + phase.String()
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
