package otelvulkan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/agentstax/vulkan/pkg/metrics"
	"github.com/agentstax/vulkan/pkg/vulkan"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestPoolConstructorGuards(t *testing.T) {
	constructors := map[string]func(context.Context, *pgxpool.Pool) error{
		"metrics": func(ctx context.Context, pool *pgxpool.Pool) error {
			_, err := NewMetrics(ctx, pool, nil)
			return err
		},
		"exporter": func(ctx context.Context, pool *pgxpool.Pool) error {
			_, err := NewExporter(ctx, pool, nil)
			return err
		},
	}
	pool, err := pgxpool.New(context.Background(), "postgres://localhost/unused?pool_min_conns=0")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for name, construct := range constructors {
		t.Run(name, func(t *testing.T) {
			if err := construct(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "pool must not be nil") {
				t.Fatalf("nil pool: %v", err)
			}
			if err := construct(ctx, pool); !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled construction: %v", err)
			}
		})
	}
}

func TestPoolIntegration(t *testing.T) {
	url := os.Getenv("VULKAN_OTEL_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set VULKAN_OTEL_TEST_DATABASE_URL to run the isolated-schema integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := fmt.Sprintf("otel_pool_test_%d", time.Now().UnixNano())
	defer func() {
		cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelCleanup()
		if _, err := pool.Exec(cleanupCtx, "DROP SCHEMA IF EXISTS "+pgx.Identifier{schema}.Sanitize()+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	client, err := vulkan.NewClient(ctx, pool, &vulkan.ClientConfig{Schema: schema})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.System().Register(ctx, nil); err != nil {
		t.Fatal(err)
	}
	instance, err := client.System().Metrics().Producer().Register(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	measurement, err := vulkan.NewMeasurement("otel_pool_probe", metrics.MetricKindGauge, 3, "", nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := instance.Produce(ctx, measurement); err != nil {
		t.Fatal(err)
	}
	measurement, err = vulkan.NewMeasurement("otel_pool_probe", metrics.MetricKindGauge, 7, "", nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	measurement.Metadata = json.RawMessage(`{"workers":["metadata_probe_worker"]}`)
	if _, err := instance.Produce(ctx, measurement); err != nil {
		t.Fatal(err)
	}
	series := client.System().Metrics().Metric(measurement.Name, measurement.Attributes)
	history, err := series.History(ctx, 10)
	if err != nil || len(history) != 2 {
		t.Fatalf("retained measurement history: %d, %v", len(history), err)
	}
	consumer, err := client.System().Metrics().Consumer("otel.probe").Register(ctx, &vulkan.ConsumerConfig{
		Start:    vulkan.Beginning(),
		Bindings: []string{measurement.Name},
	})
	if err != nil {
		t.Fatal(err)
	}
	consumeCtx, stop := context.WithCancel(ctx)
	defer stop()
	received := make(chan float64, 1)
	finished := make(chan error, 1)
	go func() {
		finished <- consumer.Consume(consumeCtx, func(ctx context.Context, message *vulkan.Measurement) error {
			select {
			case received <- message.Value:
			default:
			}
			return nil
		}, nil)
	}()
	select {
	case value := <-received:
		if value != 7 {
			t.Errorf("consumed value %v", value)
		}
	case err := <-finished:
		t.Fatalf("consumer stopped before delivery: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	stop()
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	observations, err := NewMetrics(ctx, pool, &MetricsConfig{Schema: schema})
	if err != nil {
		t.Fatal(err)
	}
	reader := sdkmetric.NewManualReader(sdkmetric.WithProducer(observations))
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer provider.Shutdown(ctx)
	var collected metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &collected); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, scope := range collected.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name == "otel_pool_probe" {
				gauge, ok := metric.Data.(metricdata.Gauge[float64])
				found = ok && len(gauge.DataPoints) == 1 && gauge.DataPoints[0].Value == 7 && gauge.DataPoints[0].Attributes.Len() == 0
			}
		}
	}
	if !found {
		t.Fatal("custom-schema measurement missing from collection")
	}
	newMeasurement, err := vulkan.NewMeasurement("new_name_after_reader_creation", metrics.MetricKindGauge, 11, "", nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := instance.Produce(ctx, newMeasurement); err != nil {
		t.Fatal(err)
	}
	if err := reader.Collect(ctx, &collected); err != nil {
		t.Fatal(err)
	}
	found = false
	for _, scope := range collected.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name == newMeasurement.Name {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("new name required instrument discovery")
	}
	canceledCtx, cancelCollection := context.WithCancel(ctx)
	cancelCollection()
	if _, err := observations.Produce(canceledCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled collection = %v", err)
	}
	exporter, err := NewExporter(ctx, pool, &ExporterConfig{Schema: schema})
	if err != nil {
		t.Fatal(err)
	}
	defer exporter.Close(ctx)
	response := httptest.NewRecorder()
	exporter.Handler().ServeHTTP(response, httptest.NewRequest("GET", "/metrics", nil).WithContext(ctx))
	if response.Code != 200 || !strings.Contains(response.Body.String(), "otel_pool_probe 7") {
		t.Fatalf("scrape: status %d, body %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "metadata_probe_worker") {
		t.Fatal("measurement metadata must not become scrape labels")
	}
	if err := exporter.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("exporter closed caller's pool: %v", err)
	}
}
