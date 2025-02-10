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
	// 测试禁用状态下的初始化
	cfg := config.OpenTelemetryConfig{Enabled: false}
	tel, err := telemetry.NewTelemetry(context.Background(), cfg)

	require.NoError(t, err)
	assert.IsType(t, &sdktrace.TracerProvider{}, tel.GetTracerProvider())
	assert.IsType(t, &sdkmetric.MeterProvider{}, tel.GetMeterProvider())

	// 验证全局provider未被修改
	assert.NotSame(t, tel.GetTracerProvider(), otel.GetTracerProvider())
	assert.NotSame(t, tel.GetMeterProvider(), otel.GetMeterProvider())
}

func TestNewTelemetry_Enabled(t *testing.T) {
	// 测试启用状态下的初始化（需要有效的endpoint）
	cfg := config.OpenTelemetryConfig{
		Enabled:     true,
		ServiceName: "test-service",
		Endpoint:    "localhost:4317",
		Metrics:     config.MetricConfig{Interval: time.Second},
	}

	tel, err := telemetry.NewTelemetry(context.Background(), cfg)

	require.NoError(t, err)
	assert.NotNil(t, tel.GetMeter())

	// 验证全局provider设置
	assert.Same(t, tel.GetTracerProvider(), otel.GetTracerProvider())
	assert.Same(t, tel.GetMeterProvider(), otel.GetMeterProvider())
}

func TestNewTelemetry_InvalidConfig(t *testing.T) {
	// 测试无效配置的情况
	t.Run("invalid resource", func(t *testing.T) {
		cfg := config.OpenTelemetryConfig{
			Enabled:     true,
			ServiceName: "", // 无效的服务名
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
		// 创建已关闭的provider来触发错误
		cfg := config.OpenTelemetryConfig{Enabled: false}
		tel, _ := telemetry.NewTelemetry(context.Background(), cfg)
		tel.Shutdown(context.Background()) // 第一次正常关闭

		err := tel.Shutdown(context.Background()) // 第二次关闭应报错
		assert.ErrorContains(t, err, "telemetry shutdown errors")
	})
}

func TestGlobalPropagators(t *testing.T) {
	// 验证全局传播器设置
	cfg := config.OpenTelemetryConfig{Enabled: true}
	telemetry.NewTelemetry(context.Background(), cfg)

	propagator := otel.GetTextMapPropagator()
	assert.Contains(t, propagator.Fields(), "traceparent")
	assert.Contains(t, propagator.Fields(), "baggage")
}
