package config

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

var validIdentifier = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

const (
	DefaultInterval    = 3 * time.Minute
	DefaultMetricsPort = 9363
)

// Config is the top-level configuration for clickhouse-shard-health.
type Config struct {
	// Interval between health check runs.
	Interval time.Duration `yaml:"interval"`

	// MetricsPort is the port to expose Prometheus metrics on.
	MetricsPort int `yaml:"metricsPort"`

	// Clusters defines one or more ClickHouse clusters to monitor.
	Clusters []ClusterEntry `yaml:"clusters"`
}

// ClusterEntry represents a single ClickHouse cluster to monitor.
type ClusterEntry struct {
	// Name is the ClickHouse cluster name (as in system.clusters).
	Name string `yaml:"name"`

	// DSN is the ClickHouse connection string.
	// Supports env var expansion: ${CLICKHOUSE_DSN}
	DSN string `yaml:"dsn"`

	// Database to monitor for replica health checks.
	Database string `yaml:"database"`
}

// Load reads configuration from a YAML file, then applies environment variable overrides.
func Load(path string) (*Config, error) {
	cfg := &Config{
		Interval:    DefaultInterval,
		MetricsPort: DefaultMetricsPort,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	expanded := os.ExpandEnv(string(data))

	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if len(cfg.Clusters) == 0 {
		return nil, fmt.Errorf("at least one cluster must be configured")
	}

	for i, c := range cfg.Clusters {
		if c.Name == "" {
			return nil, fmt.Errorf("cluster[%d]: name is required", i)
		}

		if !validIdentifier.MatchString(c.Name) {
			return nil, fmt.Errorf("cluster[%d]: name %q contains invalid characters (only alphanumeric, hyphens, and underscores are allowed)", i, c.Name)
		}

		if c.DSN == "" {
			return nil, fmt.Errorf("cluster[%d] (%s): dsn is required", i, c.Name)
		}

		if c.Database == "" {
			return nil, fmt.Errorf("cluster[%d] (%s): database is required", i, c.Name)
		}
	}

	return cfg, nil
}
