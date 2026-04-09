package health

import (
	"context"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Checker performs ClickHouse shard health checks across one or more clusters.
type Checker struct {
	clusters []ClusterConfig
	logger   *slog.Logger
	metrics  *metrics
}

// Option configures a Checker.
type Option func(*Checker)

// WithLogger sets a custom slog.Logger.
func WithLogger(logger *slog.Logger) Option {
	return func(c *Checker) {
		c.logger = logger
	}
}

// WithRegisterer sets a custom prometheus.Registerer instead of the default registry.
func WithRegisterer(reg prometheus.Registerer) Option {
	return func(c *Checker) {
		c.metrics = newMetrics(reg)
	}
}

// NewChecker creates a new Checker for the given clusters.
func NewChecker(clusters []ClusterConfig, opts ...Option) *Checker {
	c := &Checker{
		clusters: clusters,
		logger:   slog.Default(),
	}
	for _, opt := range opts {
		opt(c)
	}
	// If no custom registerer was provided, use the default.
	if c.metrics == nil {
		c.metrics = newMetrics(prometheus.DefaultRegisterer)
	}
	return c
}

// CheckAll executes all health checks once across all configured clusters.
func (c *Checker) CheckAll(ctx context.Context) {
	c.metrics.resetTransient()
	for _, cluster := range c.clusters {
		c.logger.Info("running ClickHouse shard health check", "cluster", cluster.ClusterName)
		c.checkNodeReachability(ctx, cluster)
		c.checkReplicaHealth(ctx, cluster)
		c.checkStuckReplicationQueue(ctx, cluster)
		c.checkStuckMutations(ctx, cluster)
		c.checkDDLQueueStatus(ctx, cluster)
	}
}

// Run starts periodic health checks at the given interval.
// It blocks until the context is cancelled.
func (c *Checker) Run(ctx context.Context, interval time.Duration) error {
	c.logger.Info("starting ClickHouse shard health checker", "interval", interval, "clusters", len(c.clusters))

	// Run immediately on start.
	c.CheckAll(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("ClickHouse shard health checker stopped")
			return ctx.Err()
		case <-ticker.C:
			c.CheckAll(ctx)
		}
	}
}
