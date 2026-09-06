package otelvulkan

import (
	"context"
	"errors"
	"net/http"

	"github.com/agentstax/vulkan/pkg/common/logging"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	otelprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// Exporter serves the measurements as a Prometheus /metrics endpoint: its own
// Metrics bound to a provider whose only reader is the otel Prometheus
// reader -- every scrape drives the observation callback, so /metrics
// serves values read at scrape time, never a cache.
type Exporter struct {
	Config *ExporterConfig
	Logger logging.Logger

	metrics  *Metrics
	provider *sdkmetric.MeterProvider
	registry *prometheus.Registry
}

// NewExporter pings pool using ctx and builds its own datastore.
// The caller owns the pool; cfg may be nil or sparse.
func NewExporter(ctx context.Context, pool *pgxpool.Pool, cfg *ExporterConfig) (*Exporter, error) {
	if pool == nil {
		return nil, errors.New("pool must not be nil")
	}
	if cfg == nil {
		cfg = &ExporterConfig{}
	}
	cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	registry := prometheus.NewRegistry()
	// one meter source feeds the registry, so per-series otel_scope_* labels
	// carry no information -- dropped
	reader, err := otelprometheus.New(otelprometheus.WithRegisterer(registry), otelprometheus.WithoutScopeInfo())
	if err != nil {
		return nil, err
	}
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	exporterMetrics, err := NewMetrics(ctx, pool, &MetricsConfig{
		Schema:         cfg.Schema,
		Meter:          provider.Meter(meterScopeName),
		CollectTimeout: cfg.CollectTimeout,
		Logger:         cfg.Logger,
		Retry:          cfg.Retry,
	})
	if err != nil {
		_ = provider.Shutdown(ctx)
		return nil, err
	}

	return &Exporter{
		Config:   cfg,
		Logger:   exporterMetrics.Logger,
		metrics:  exporterMetrics,
		provider: provider,
		registry: registry,
	}, nil
}

// Handler serves the Prometheus /metrics endpoint. Each request first
// registers instruments for metric names that appeared since the last
// scrape, then the reader's collection runs the observation callback.
// A failed registration pass still serves the last instrument set.
func (e *Exporter) Handler() http.Handler {
	serve := promhttp.HandlerFor(e.registry, promhttp.HandlerOpts{})
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := e.metrics.RegisterMetricInstruments(request.Context()); err != nil {
			e.Logger.WarnContext(request.Context(), "could not register metric instrument", "error", err)
		}
		serve.ServeHTTP(writer, request)
	})
}

// RegisterMetricInstruments runs the registration pass. Handler runs it per
// scrape; call it directly to fail fast at startup instead of on the first
// scrape.
// Returns ErrTopicNotFound until RegisterSystem has run.
func (e *Exporter) RegisterMetricInstruments(ctx context.Context) error {
	return e.metrics.RegisterMetricInstruments(ctx)
}

// Close shuts the meter provider down; the registry stops receiving updates.
func (e *Exporter) Close(ctx context.Context) error {
	return e.provider.Shutdown(ctx)
}
