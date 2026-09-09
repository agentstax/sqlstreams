package producer

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/agentstax/vulkan/pkg/metric"
	"github.com/agentstax/vulkan/pkg/produce"
)

// MetricProducerInstance produces custom measurements with routing and
// compaction keys derived from their metric name and attributes.
type MetricProducerInstance struct {
	instance *ProducerInstance[metric.Measurement]
}

func newMetricProducerInstance(instance *ProducerInstance[metric.Measurement]) (*MetricProducerInstance, error) {
	if instance == nil {
		return nil, errors.New("instance must not be nil")
	}
	return &MetricProducerInstance{instance: instance}, nil
}

// RegisterMetrics resolves the system metrics topic -- the client spells it
// System().Metrics().Producer().Register. cfg may be nil or sparse.
// Register the system first; this does not create it.
func (p *Producer) RegisterMetrics(ctx context.Context, cfg *ProducerConfig) (*MetricProducerInstance, error) {
	instance, err := p.Register[metric.Measurement](ctx, metric.MetricTopicName, cfg)
	if err != nil {
		return nil, err
	}
	return newMetricProducerInstance(instance)
}

// Produce retains the newest measurement per series while keeping its history.
// Names beginning with "vulkan." are reserved for built-in measurements.
func (p *MetricProducerInstance) Produce(ctx context.Context, measurement *metric.Measurement) (*ProduceResult[metric.Measurement], error) {
	if measurement == nil {
		return nil, errors.New("measurement must not be nil")
	}
	if strings.HasPrefix(measurement.Name, metric.MetricNameReservedPrefix) {
		return nil, fmt.Errorf("metric name %q uses the %q prefix, reserved for Vulkan's own metrics", measurement.Name, metric.MetricNameReservedPrefix)
	}

	return p.instance.Produce(ctx, measurement, &produce.ProduceOptions{
		RoutingKey: measurement.Name,
		MessageKey: metric.MeasurementKey(measurement.Name, measurement.Attributes),
		Compaction: &produce.CompactionOptions{Enable: true},
	})
}
