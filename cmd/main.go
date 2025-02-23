package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nexus/internal/config"
	"nexus/internal/healthcheck"
	lg "nexus/internal/logger"
	px "nexus/internal/proxy"
	"nexus/internal/route"
	"nexus/internal/telemetry"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

func main() {
	// Load configuration
	cfg := config.NewConfig()
	if err := config.Validate("configs/config.yaml"); err != nil {
		log.Fatalf("config error - %v", err)
	}
	if err := cfg.LoadFromFile("configs/config.yaml"); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize configuration watcher
	configWatcher := config.NewConfigWatcher("configs/config.yaml")

	// Initialize logger (using singleton mode)
	logger := lg.GetInstance()
	if cfg.GetLogLevel() != "" {
		logger.SetLevel(logger.ToLogLevel(cfg.GetLogLevel()))
	}

	// Initialize health checker
	healthCheckCfg := cfg.GetHealthCheckConfig()
	healthChecker := healthcheck.NewHealthChecker(healthCheckCfg.Interval, healthCheckCfg.Timeout)
	for _, server := range cfg.Services {
		for _, s := range server.Servers {
			healthChecker.AddServer(s.Address)
		}
	}
	go healthChecker.Start()
	defer healthChecker.Stop()

	// Initialize reverse proxy
	router := route.NewRouter(cfg.Routes, cfg.Services)
	proxy := px.NewProxy(router)
	proxy.SetErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Error("Proxy error: %v", err)
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
	})

	// Initialize OpenTelemetry
	tel, err := telemetry.NewTelemetry(context.Background(), cfg.Telemetry.OpenTelemetry)
	if err != nil {
		log.Fatalf("failed to initialize telemetry: %v", err)
	}
	defer tel.Shutdown(context.Background())

	// 配置追踪传播器
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))

	// Register configuration update callback
	configWatcher.Watch(func(newCfg *config.Config) {
		logger.Info("Configuration changed, applying updates...")

		// Update routes
		router.Update(newCfg.Routes, newCfg.Services)

		// Update health check
		healthChecker.UpdateInterval(newCfg.GetHealthCheckConfig().Interval)
		healthChecker.UpdateTimeout(newCfg.GetHealthCheckConfig().Timeout)

		// Update log level
		logger.SetLevel(logger.ToLogLevel(newCfg.GetLogLevel()))
	})

	// Start configuration watcher
	configWatcher.Start()

	// Start HTTP server
	server := &http.Server{
		Addr:    cfg.GetListenAddr(),
		Handler: proxy,
	}

	go func() {
		logger.Info("Starting server on %s", cfg.GetListenAddr())
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server shutdown error: %v", err)
	}
	logger.Info("Server exited")
}
