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
	// 加载配置
	cfg := internal.NewConfig()
	if err := cfg.LoadFromFile("configs/config.yaml"); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化日志
	logger := internal.NewLogger(internal.LevelInfo)
	if cfg.GetLogLevel() == "debug" {
		logger.SetLevel(internal.LevelDebug)
	}

	// 初始化负载均衡器
	balancer := internal.NewRoundRobinBalancer()
	for _, server := range cfg.GetServers() {
		balancer.Add(server)
	}

	// 初始化健康检查
	healthCheckCfg := cfg.GetHealthCheckConfig()
	healthChecker := internal.NewHealthChecker(healthCheckCfg.Interval, healthCheckCfg.Timeout)
	for _, server := range cfg.GetServers() {
		healthChecker.AddServer(server)
	}
	go healthChecker.Start()
	defer healthChecker.Stop()

	// 初始化反向代理
	proxy := internal.NewProxy(balancer)
	proxy.SetErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Error("Proxy error: %v", err)
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
	})

	// 启动 HTTP 服务器
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

	// 优雅关闭
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
