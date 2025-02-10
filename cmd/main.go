package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	lb "nexus/internal/balancer"
	"nexus/internal/config"
	"nexus/internal/healthcheck"
	lg "nexus/internal/logger"
	px "nexus/internal/proxy"
	"nexus/internal/telemetry"
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

	// Initialize load balancer
	balancer := lb.NewBalancer(cfg.GetBalancerType())
	for _, server := range cfg.GetServers() {
		if cfg.GetBalancerType() == "weighted_round_robin" {
			if wrr, ok := balancer.(*lb.WeightedRoundRobinBalancer); ok {
				wrr.AddWithWeight(server.Address, server.Weight)
			}
		} else {
			balancer.Add(server.Address)
		}
	}

	// Initialize health checker
	healthCheckCfg := cfg.GetHealthCheckConfig()
	healthChecker := healthcheck.NewHealthChecker(healthCheckCfg.Interval, healthCheckCfg.Timeout)
	for _, server := range cfg.GetServers() {
		healthChecker.AddServer(server.Address)
	}
	go healthChecker.Start()
	defer healthChecker.Stop()

	// Initialize reverse proxy
	proxy := px.NewProxy(balancer)
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

	// Register configuration update callback
	configWatcher.Watch(func(newCfg *config.Config) {
		logger.Info("Configuration changed, applying updates...")

		// Update load balancer
		balancer.UpdateServers(newCfg.GetServers())

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
