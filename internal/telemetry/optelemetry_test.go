package telemetry_test

import (
	"context"
	"testing"
	"time"

	"nexus/internal/config"
	"nexus/internal/telemetry"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestNewTelemetry_Disabled(t *testing.T) {
	// Test initialization in disabled state
	cfg := config.OpenTelemetryConfig{Enabled: false}
	tel, err := telemetry.NewTelemetry(context.Background(), cfg)

	require.NoError(t, err)
	assert.IsType(t, &sdktrace.TracerProvider{}, tel.GetTracerProvider())
	assert.IsType(t, &sdkmetric.MeterProvider{}, tel.GetMeterProvider())

	// Verify that the global provider was not modified
	assert.NotSame(t, tel.GetTracerProvider(), otel.GetTracerProvider())
	assert.NotSame(t, tel.GetMeterProvider(), otel.GetMeterProvider())
}

func TestNewTelemetry_Enabled(t *testing.T) {
	// Test initialization in enabled state (requires a valid endpoint)
	cfg := config.OpenTelemetryConfig{
		Enabled:     true,
		ServiceName: "test-service",
		Endpoint:    "localhost:4317",
		Metrics:     config.MetricConfig{Interval: time.Second},
	}

	tel, err := telemetry.NewTelemetry(context.Background(), cfg)

	require.NoError(t, err)
	assert.NotNil(t, tel.GetMeter())

	// Verify that the global provider is set
	assert.Same(t, tel.GetTracerProvider(), otel.GetTracerProvider())
	assert.Same(t, tel.GetMeterProvider(), otel.GetMeterProvider())
}

func TestNewTelemetry_InvalidConfig(t *testing.T) {
	// Test invalid configuration
	t.Run("invalid resource", func(t *testing.T) {
		cfg := config.OpenTelemetryConfig{
			Enabled:     true,
			ServiceName: "", // Invalid service name
		}
		_, err := telemetry.NewTelemetry(context.Background(), cfg)
		assert.ErrorContains(t, err, "service name is required")
	})

	t.Run("invalid endpoint", func(t *testing.T) {
		cfg := config.OpenTelemetryConfig{
			Enabled:     true,
			ServiceName: "test",
			Endpoint:    "invalid-endpoint",
		}
		_, err := telemetry.NewTelemetry(context.Background(), cfg)
		assert.ErrorContains(t, err, "invalid endpoint format")
	})
}

func TestShutdown(t *testing.T) {
	t.Run("normal shutdown", func(t *testing.T) {
		cfg := config.OpenTelemetryConfig{Enabled: false}
		tel, _ := telemetry.NewTelemetry(context.Background(), cfg)
		assert.NoError(t, tel.Shutdown(context.Background()))
	})

	t.Run("shutdown with errors", func(t *testing.T) {
		// Create a closed provider to trigger an error
		cfg := config.OpenTelemetryConfig{Enabled: false}
		tel, _ := telemetry.NewTelemetry(context.Background(), cfg)
		tel.Shutdown(context.Background()) // First normal shutdown

		err := tel.Shutdown(context.Background()) // Second shutdown should report an error
		assert.ErrorContains(t, err, "telemetry shutdown errors")
	})
}

func TestGlobalPropagators(t *testing.T) {
	// Verify that the global propagators are set
	cfg := config.OpenTelemetryConfig{Enabled: true}
	telemetry.NewTelemetry(context.Background(), cfg)

	propagator := otel.GetTextMapPropagator()
	assert.Contains(t, propagator.Fields(), "traceparent")
	assert.Contains(t, propagator.Fields(), "baggage")
}
