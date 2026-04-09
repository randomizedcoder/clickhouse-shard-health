package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/mmtretiak/clickhouse-shard-health/config"
	"github.com/mmtretiak/clickhouse-shard-health/health"
)

var version = "dev"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	logger.Info("starting clickhouse-shard-health", "version", version)

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	clusters := make([]health.ClusterConfig, 0, len(cfg.Clusters))
	for _, entry := range cfg.Clusters {
		opts, err := clickhouse.ParseDSN(entry.DSN)
		if err != nil {
			logger.Error("failed to parse ClickHouse DSN", "error", err, "cluster", entry.Name)
			os.Exit(1)
		}

		conn, err := clickhouse.Open(opts)
		if err != nil {
			logger.Error("failed to open ClickHouse connection", "error", err, "cluster", entry.Name)
			os.Exit(1)
		}

		clusters = append(clusters, health.ClusterConfig{
			Conn:        conn,
			ClusterName: entry.Name,
			Database:    entry.Database,
		})
	}

	checker := health.NewChecker(clusters, health.WithLogger(logger))

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	metricsAddr := fmt.Sprintf(":%d", cfg.MetricsPort)
	server := &http.Server{
		Addr:              metricsAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("metrics server starting", "addr", metricsAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("metrics server failed", "error", err)
			stop()
		}
	}()

	if err := checker.Run(ctx, cfg.Interval); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("checker exited", "error", err)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)

	for _, cl := range clusters {
		_ = cl.Conn.Close()
	}
	logger.Info("shutdown complete")
}
