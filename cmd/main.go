package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nexus/internal"
)

func main() {
	// Load configuration
	cfg := internal.NewConfig()
	if err := cfg.LoadFromFile("configs/config.yaml"); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	logger := internal.NewLogger(internal.LevelInfo)
	if cfg.GetLogLevel() == "debug" {
		logger.SetLevel(internal.LevelDebug)
	}

	// Initialize load balancer
	balancer := internal.NewBalancer(cfg.GetBalancerType())
	for _, server := range cfg.GetServers() {
		if cfg.GetBalancerType() == "weighted_round_robin" {
			if wrr, ok := balancer.(*internal.WeightedRoundRobinBalancer); ok {
				wrr.AddWithWeight(server.Address, server.Weight)
			}
		} else {
			balancer.Add(server.Address)
		}
	}

	// Initialize health checker
	healthCheckCfg := cfg.GetHealthCheckConfig()
	healthChecker := internal.NewHealthChecker(healthCheckCfg.Interval, healthCheckCfg.Timeout)
	for _, server := range cfg.GetServers() {
		healthChecker.AddServer(server.Address)
	}
	go healthChecker.Start()
	defer healthChecker.Stop()

	// Initialize reverse proxy
	proxy := internal.NewProxy(balancer)
	proxy.SetErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Error("Proxy error: %v", err)
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
	})

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
