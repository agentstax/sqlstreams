package consumer

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/allegedlyreliable/sqlstreams/.bench/common"
	"github.com/allegedlyreliable/sqlstreams/.bench/record"
	"github.com/allegedlyreliable/sqlstreams/client"
)

// Instances is the consumer instances one process runs on one group,
// numbered c-1 upward. Each instance is its own Register and Consume
// session under its own ctx, so a scale-down is a graceful stop of the
// highest-numbered ones. The runner decides the count; Instances only
// moves to it. Every group's Instances in a process share one failed
// channel, so the runner has one place to wait.
type Instances struct {
	handle        *sqlstreams.ConsumerHandle[common.Order]
	cfg           *sqlstreams.ConsumerConfig
	consume       *sqlstreams.ConsumeOptions
	stream        string
	group         string
	failRate      float64
	writer        *record.HandlerWriter
	progress      *record.Progress
	handlerConfig *HandlerConfig
	name          string
	failed        chan error

	mutex   sync.Mutex
	running []*runningInstance
}

type runningInstance struct {
	stop context.CancelFunc
	done chan struct{}
}

func NewInstances(handle *sqlstreams.ConsumerHandle[common.Order], cfg *sqlstreams.ConsumerConfig, consume *sqlstreams.ConsumeOptions, stream string, group string, failRate float64, writer *record.HandlerWriter, progress *record.Progress, name string, failed chan error, handlerConfig *HandlerConfig) (*Instances, error) {
	if handle == nil {
		return nil, errors.New("handle must not be nil")
	}
	if cfg == nil {
		return nil, errors.New("cfg must not be nil")
	}
	if consume == nil {
		return nil, errors.New("consume must not be nil")
	}
	if stream == "" {
		return nil, errors.New("stream must not be empty")
	}
	if group == "" {
		return nil, errors.New("group must not be empty")
	}
	if failRate < 0 || failRate > 1 {
		return nil, fmt.Errorf("failRate must be between 0 and 1, got %g", failRate)
	}
	if writer == nil {
		return nil, errors.New("writer must not be nil")
	}
	if name == "" {
		return nil, errors.New("name must not be empty")
	}
	if failed == nil {
		return nil, errors.New("failed must not be nil")
	}
	return &Instances{handle: handle, cfg: cfg, consume: consume, stream: stream, group: group, failRate: failRate, writer: writer, progress: progress, handlerConfig: handlerConfig, name: name, failed: failed}, nil
}

// SetCount starts or stops instances until count are running. Stopping
// waits for each stopped session to return.
func (i *Instances) SetCount(ctx context.Context, count int) error {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	for len(i.running) > count {
		last := i.running[len(i.running)-1]
		last.stop()
		<-last.done
		i.running = i.running[:len(i.running)-1]
	}
	for len(i.running) < count {
		instance, err := i.start(ctx, len(i.running)+1)
		if err != nil {
			return err
		}
		i.running = append(i.running, instance)
	}
	return nil
}

func (i *Instances) start(ctx context.Context, number int) (*runningInstance, error) {
	consumerName := fmt.Sprintf("%s/%s/%s/c-%d", i.name, i.stream, i.group, number)
	handler, err := NewHandler(consumerName, i.stream, i.group, i.failRate, i.writer, i.progress, i.failed, i.handlerConfig)
	if err != nil {
		return nil, err
	}
	session, err := i.handle.Register(ctx, i.cfg)
	if err != nil {
		return nil, err
	}

	sessionCtx, stop := context.WithCancel(ctx)
	instance := &runningInstance{stop: stop, done: make(chan struct{})}
	go func() {
		defer close(instance.done)
		err := session.Consume(sessionCtx, handler.Handle, i.consume)
		if err != nil && !errors.Is(err, context.Canceled) {
			select {
			case i.failed <- fmt.Errorf("%s: %w", consumerName, err):
			default:
			}
		}
	}()
	return instance, nil
}
