package otelvulkan

// Package otelvulkan exports retained Vulkan measurements through an external
// OpenTelemetry SDK producer. Exporter serves that producer through Prometheus.

import (
	"context"
	"errors"
	"time"

	"github.com/agentstax/vulkan/pkg/common"
	"github.com/agentstax/vulkan/pkg/common/diagnostic"
	"github.com/agentstax/vulkan/pkg/common/logging"
	"github.com/agentstax/vulkan/pkg/datastore"
	"github.com/agentstax/vulkan/pkg/metrics"
	metricscontroller "github.com/agentstax/vulkan/pkg/metrics/controller"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

const meterScopeName = "github.com/agentstax/vulkan/otelvulkan"

// Metrics is an external SDK producer of the newest retained measurements.
// Attach it to a reader with sdkmetric.WithProducer; no registration pass is needed.
type Metrics struct {
	Config *MetricsConfig
	Logger logging.Logger

	measurements *metricscontroller.MetricsController
}

// NewMetrics pings pool using ctx and builds its own datastore.
// The caller owns the pool; cfg may be nil or sparse.
func NewMetrics(ctx context.Context, pool *pgxpool.Pool, cfg *MetricsConfig) (*Metrics, error) {
	if pool == nil {
		return nil, errors.New("pool must not be nil")
	}
	if cfg == nil {
		cfg = &MetricsConfig{}
	}
	cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	ds, err := datastore.NewPostgresDatastore(ctx, pool, &datastore.PostgresDatastoreConfig{
		Schema: cfg.Schema,
		Logger: cfg.Logger,
		Retry:  cfg.Retry,
	})
	if err != nil {
		return nil, err
	}
	measurements, err := metricscontroller.NewMetricsController(ds, ds.Logger)
	if err != nil {
		return nil, err
	}
	return &Metrics{Config: cfg, Logger: ds.Logger, measurements: measurements}, nil
}

// Produce reads current retained measurements within CollectTimeout.
// Readers own cadence and resources. Concurrent calls share no collection state.
// Returns migrate.ErrNotRegistered before the system metrics topic exists.
func (m *Metrics) Produce(ctx context.Context) ([]metricdata.ScopeMetrics, error) {
	ctx, cancel := context.WithTimeout(ctx, m.Config.CollectTimeout)
	defer cancel()

	rows, err := m.measurements.ListMeasurements(ctx)
	if err != nil {
		return nil, err
	}
	return []metricdata.ScopeMetrics{{
		Scope:   instrumentation.Scope{Name: meterScopeName},
		Metrics: toMetrics(rows, time.Now()),
	}}, nil
}

// ***************
// *** HELPERS ***
// ***************

func toMetrics(rows []*common.StoredMessage[metrics.Measurement], current time.Time) []metricdata.Metrics {
	positions := make(map[[3]string]int)
	collected := make([]metricdata.Metrics, 0)
	for _, row := range rows {
		measurement := row.Message
		identity := [3]string{measurement.Name, string(measurement.Kind), string(measurement.Unit)}
		position, found := positions[identity]
		if !found {
			position = len(collected)
			positions[identity] = position
			metric := metricdata.Metrics{Name: measurement.Name, Unit: string(measurement.Unit)}
			if declared, found := diagnostic.GetMetric(measurement.Name); found {
				metric.Description = declared.Description
			}
			collected = append(collected, metric)
		}
		point := metricdata.DataPoint[float64]{
			Attributes: attribute.NewSet(toAttributes(measurement.Attributes)...),
			Time:       current,
			Value:      measurement.Value,
		}
		switch measurement.Kind {
		case metrics.MetricKindCounter:
			// Retained totals have no established start time; exporter startup is not a reset.
			sum, _ := collected[position].Data.(metricdata.Sum[float64])
			sum.Temporality = metricdata.CumulativeTemporality
			sum.IsMonotonic = true
			sum.DataPoints = append(sum.DataPoints, point)
			collected[position].Data = sum
		default:
			gauge, _ := collected[position].Data.(metricdata.Gauge[float64])
			gauge.DataPoints = append(gauge.DataPoints, point)
			collected[position].Data = gauge
		}
	}
	return collected
}

func toAttributes(attributes map[string]string) []attribute.KeyValue {
	pairs := make([]attribute.KeyValue, 0, len(attributes))
	for key, value := range attributes {
		pairs = append(pairs, attribute.String(key, value))
	}
	return pairs
}
