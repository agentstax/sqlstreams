package producer

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/agentstax/vulkan/pkg/metrics"
	"github.com/agentstax/vulkan/pkg/produce"
)

// MetricsProducerInstance produces custom measurements with routing and
// compaction keys derived from their metric name and attributes.
type MetricsProducerInstance struct {
	instance *ProducerInstance[metrics.Measurement]
}

func newMetricsProducerInstance(instance *ProducerInstance[metrics.Measurement]) (*MetricsProducerInstance, error) {
	if instance == nil {
		return nil, errors.New("instance must not be nil")
	}
	return &MetricsProducerInstance{instance: instance}, nil
}

// RegisterMetrics resolves the system metrics topic. cfg may be nil or sparse.
// Register the system first; this does not create it.
func (p *Producer) RegisterMetrics(ctx context.Context, cfg *ProducerConfig) (*MetricsProducerInstance, error) {
	instance, err := p.Register[metrics.Measurement](ctx, metrics.MetricsTopicName, cfg)
	if err != nil {
		return nil, err
	}
	return newMetricsProducerInstance(instance)
}

// Produce retains the newest measurement per series while keeping its history.
// Names beginning with "vulkan." are reserved for built-in measurements.
func (p *MetricsProducerInstance) Produce(ctx context.Context, measurement *metrics.Measurement) (*ProduceResult[metrics.Measurement], error) {
	if measurement == nil {
		return nil, errors.New("measurement must not be nil")
	}
	if strings.HasPrefix(measurement.Name, metrics.MetricNameReservedPrefix) {
		return nil, fmt.Errorf("metric name %q uses the %q prefix, reserved for Vulkan's own metrics", measurement.Name, metrics.MetricNameReservedPrefix)
	}

	compaction, err := produce.NewCompactionOptions(0)
	if err != nil {
		return nil, err
	}

	return p.instance.Produce(ctx, measurement, &produce.ProduceOptions{
		RoutingKey: measurement.Name,
		MessageKey: metrics.MeasurementKey(measurement.Name, measurement.Attributes),
		Compaction: compaction,
	})
}
