package consumer

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/agentstax/vulkan/bench/reliability/common"
	"github.com/agentstax/vulkan/bench/reliability/ledger"
	vulkan "github.com/agentstax/vulkan/pkg/vulkan"
)

// Instances is the verifiable consumer fleet one container runs, numbered
// c-1 upward. Each instance is its own Register and Consume session under its
// own ctx, so a scale-down is a graceful stop of the highest-numbered ones.
// The runner decides the count; Instances only moves to it.
type Instances struct {
	handle   *vulkan.ConsumerHandle[common.Order]
	cfg      *vulkan.ConsumerConfig
	group    string
	failRate float64
	handled  *ledger.Writer
	name     string

	mutex   sync.Mutex
	running []*runningInstance
	errs    chan error
}

type runningInstance struct {
	stop context.CancelFunc
	done chan struct{}
}

func NewInstances(handle *vulkan.ConsumerHandle[common.Order], cfg *vulkan.ConsumerConfig, group string, failRate float64, handled *ledger.Writer, name string) (*Instances, error) {
	if handle == nil {
		return nil, errors.New("handle must not be nil")
	}
	if cfg == nil {
		return nil, errors.New("cfg must not be nil")
	}
	if group == "" {
		return nil, errors.New("group must not be empty")
	}
	if failRate < 0 || failRate > 1 {
		return nil, fmt.Errorf("failRate must be between 0 and 1, got %g", failRate)
	}
	if handled == nil {
		return nil, errors.New("handled must not be nil")
	}
	if name == "" {
		return nil, errors.New("name must not be empty")
	}
	return &Instances{handle: handle, cfg: cfg, group: group, failRate: failRate, handled: handled, name: name, errs: make(chan error, 1)}, nil
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

// Failed reports the first Consume session that returned an error other
// than its own cancellation; a healthy fleet never sends.
func (i *Instances) Failed() <-chan error {
	return i.errs
}

func (i *Instances) start(ctx context.Context, number int) (*runningInstance, error) {
	consumerName := fmt.Sprintf("%s/c-%d", i.name, number)
	handler, err := NewHandler(consumerName, i.group, i.failRate, i.handled)
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
		err := session.Consume(sessionCtx, handler.Handle, nil)
		if err != nil && !errors.Is(err, context.Canceled) {
			select {
			case i.errs <- fmt.Errorf("%s: %w", consumerName, err):
			default:
			}
		}
	}()
	return instance, nil
}
