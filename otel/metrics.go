package otel

// Package otel exports retained SQLStreams measurements through an external
// OpenTelemetry SDK producer. Exporter serves that producer through Prometheus.

import (
	"context"
	"errors"
	"maps"
	"slices"

	"github.com/agentstax/sqlstreams/pkg/common"
	"github.com/agentstax/sqlstreams/pkg/common/logging"
	"github.com/agentstax/sqlstreams/pkg/datastore"
	"github.com/agentstax/sqlstreams/pkg/metric"
	metricscontroller "github.com/agentstax/sqlstreams/pkg/metric/controller"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

const meterScopeName = "github.com/agentstax/sqlstreams/otel"

// Metrics is an external SDK producer of the newest retained measurements.
// Attach it to a reader with sdkmetric.WithProducer; no registration pass is needed.
type Metrics struct {
	Config *MetricsConfig
	Logger logging.Logger

	measurements *metricscontroller.MetricController
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
// Read errors accompany read-success 0, including migrate.ErrNotRegistered before setup.
// Rejected families are reported without failing collection of healthy families.
func (m *Metrics) Produce(ctx context.Context) ([]metricdata.ScopeMetrics, error) {
	ctx, cancel := context.WithTimeout(ctx, m.Config.CollectTimeout)
	defer cancel()

	rows, err := m.measurements.ListMeasurements(ctx)
	if err != nil {
		return toCollection(nil, 0, false), err
	}

	accepted := m.filterMeasurements(ctx, rows)
	return toCollection(accepted, len(rows), true), nil
}

func (m *Metrics) filterMeasurements(ctx context.Context, rows []*common.StoredMessage[metric.Measurement]) []*common.StoredMessage[metric.Measurement] {
	// Group observations by the original name: validation accepts or rejects a whole family.
	families := make(map[string][]*common.StoredMessage[metric.Measurement])
	for _, row := range rows {
		families[row.Message.Name] = append(families[row.Message.Name], row)
	}
	rejected := rejectedFamilies(families)

	// Keep valid families and report why the others were omitted.
	accepted := make([]*common.StoredMessage[metric.Measurement], 0, len(rows))
	for _, name := range slices.Sorted(maps.Keys(families)) {
		family := families[name]
		if reason, found := rejected[name]; found {
			m.Logger.WarnContext(ctx, metric.EventMeasurementsCannotBeExported.Message(),
				"code", metric.EventMeasurementsCannotBeExported.GetCode(),
				"metric_names", []string{name}, "detail", reason)
			continue
		}
		accepted = append(accepted, family...)
	}
	return accepted
}
