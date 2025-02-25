package telemetry

import (
	"context"
	"fmt"
	"net"
	"nexus/internal/config"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

type Telemetry struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	meter          otelmetric.Meter
}

func NewTelemetry(ctx context.Context, cfg config.OpenTelemetryConfig) (*Telemetry, error) {
	if !cfg.Enabled {
		return &Telemetry{
			tracerProvider: sdktrace.NewTracerProvider(),
			meterProvider:  sdkmetric.NewMeterProvider(),
		}, nil
	}

	if cfg.ServiceName == "" {
		return nil, fmt.Errorf("service name is required")
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Initialize Trace exporter
	traceExporter, err := newTraceExporter(ctx, cfg.Endpoint)
	if err != nil {
		return nil, err
	}

	// Initialize Metric exporter
	metricExporter, err := newMetricExporter(ctx, cfg.Endpoint)
	if err != nil {
		return nil, err
	}

	// Create Trace Provider
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)

	// Create Metric Provider
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter,
			sdkmetric.WithInterval(cfg.Metrics.Interval),
		)),
		sdkmetric.WithResource(res),
	)

	// Set global Provider
	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Telemetry{
		tracerProvider: tracerProvider,
		meterProvider:  meterProvider,
		meter:          meterProvider.Meter(cfg.ServiceName),
	}, nil
}

func newTraceExporter(ctx context.Context, endpoint string) (sdktrace.SpanExporter, error) {
	// Validate endpoint format
	if _, _, err := net.SplitHostPort(endpoint); err != nil {
		return nil, fmt.Errorf("invalid endpoint format: %w", err)
	}

	return otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
}

func newMetricExporter(ctx context.Context, endpoint string) (sdkmetric.Exporter, error) {
	return otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(endpoint),
		otlpmetricgrpc.WithInsecure(),
	)
}

func (t *Telemetry) Shutdown(ctx context.Context) error {
	var errs []error
	if err := t.tracerProvider.Shutdown(ctx); err != nil {
		errs = append(errs, err)
	}
	if err := t.meterProvider.Shutdown(ctx); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("telemetry shutdown errors: %v", errs)
	}
	return nil
}

func (t *Telemetry) GetTracerProvider() *sdktrace.TracerProvider {
	return t.tracerProvider
}

func (t *Telemetry) GetMeterProvider() *sdkmetric.MeterProvider {
	return t.meterProvider
}

func (t *Telemetry) GetMeter() otelmetric.Meter {
	return t.meter
}
