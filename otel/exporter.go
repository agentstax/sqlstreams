package otel

import (
	"context"
	"errors"
	"net/http"

	"github.com/agentstax/vulkan/pkg/common/logging"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/otlptranslator"
	otelprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// Exporter serves retained measurements through its private OTel Prometheus reader.
// Each scrape reads current retained values without an instrument registration pass.
type Exporter struct {
	Config *ExporterConfig
	Logger logging.Logger

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

	exporterMetrics, err := NewMetrics(ctx, pool, &MetricsConfig{
		Schema:         cfg.Schema,
		CollectTimeout: cfg.CollectTimeout,
		Logger:         cfg.Logger,
		Retry:          cfg.Retry,
	})
	if err != nil {
		return nil, err
	}
	registry := prometheus.NewRegistry()
	reader, err := otelprometheus.New(
		otelprometheus.WithRegisterer(registry),
		otelprometheus.WithoutScopeInfo(),
		otelprometheus.WithProducer(exporterMetrics),
		otelprometheus.WithTranslationStrategy(otlptranslator.UnderscoreEscapingWithSuffixes),
	)
	if err != nil {
		return nil, err
	}
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	return &Exporter{
		Config:   cfg,
		Logger:   exporterMetrics.Logger,
		provider: provider,
		registry: registry,
	}, nil
}

// Handler serves the Prometheus /metrics endpoint; its reader drives collection.
func (e *Exporter) Handler() http.Handler {
	return promhttp.HandlerFor(e.registry, promhttp.HandlerOpts{})
}

// Close shuts down the private meter provider, not the HTTP server or caller's pool.
// In-flight scrapes can finish after Close returns. Drain the HTTP server first;
// close the pool only after its users have stopped.
func (e *Exporter) Close(ctx context.Context) error {
	return e.provider.Shutdown(ctx)
}
