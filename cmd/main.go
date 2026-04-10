// Package main is the entry point for clickhouse-shard-health.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/pprof"
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

func pprofEnabled() bool {
	if os.Getenv("PPROF_ENABLED") != "" {
		return true
	}
	enabled := flag.Bool("pprof", false, "enable pprof profiling endpoints at /debug/pprof/")
	flag.Parse()
	return *enabled
}

func main() {
	enablePprof := pprofEnabled()

	var logLevel slog.LevelVar
	logLevel.Set(slog.LevelInfo)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: &logLevel}))
	slog.SetDefault(logger)

	logger.Info("starting clickhouse-shard-health", "version", version, "pprof", enablePprof)

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	clusters, clusterErr := buildClusters(cfg.Clusters, logger)
	if clusterErr != nil {
		os.Exit(1)
	}

	checker := health.NewChecker(clusters, health.WithLogger(logger))

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "ok")
	})

	if enablePprof {
		logger.Info("pprof endpoints enabled at /debug/pprof/")
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}

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
		if listenErr := server.ListenAndServe(); listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			logger.Error("metrics server failed", "error", listenErr)
			stop()
		}
	}()

	if runErr := checker.Run(ctx, cfg.Interval); runErr != nil && !errors.Is(runErr, context.Canceled) {
		logger.Error("checker exited", "error", runErr)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
		logger.Error("server shutdown failed", "error", shutdownErr)
	}

	for _, cl := range clusters {
		if closeErr := cl.Conn.Close(); closeErr != nil {
			logger.Error("failed to close connection", "error", closeErr, "cluster", cl.ClusterName)
		}
	}

	logger.Info("shutdown complete")
}

func buildClusters(entries []config.ClusterEntry, logger *slog.Logger) ([]health.ClusterConfig, error) {
	clusters := make([]health.ClusterConfig, 0, len(entries))

	for _, entry := range entries {
		opts, parseErr := clickhouse.ParseDSN(entry.DSN)
		if parseErr != nil {
			logger.Error("failed to parse ClickHouse DSN", "error", parseErr, "cluster", entry.Name)
			return nil, parseErr
		}

		conn, openErr := clickhouse.Open(opts)
		if openErr != nil {
			logger.Error("failed to open ClickHouse connection", "error", openErr, "cluster", entry.Name)
			return nil, openErr
		}

		clusters = append(clusters, health.ClusterConfig{
			Conn:        conn,
			ClusterName: entry.Name,
			Database:    entry.Database,
		})
	}

	return clusters, nil
}
