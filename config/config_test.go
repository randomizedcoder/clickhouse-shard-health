package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestUnitLoad_ValidConfig(t *testing.T) {
	path := writeTemp(t, `
interval: 5m
metricsPort: 9090
clusters:
  - name: default
    dsn: "clickhouse://localhost:9000"
    database: mydb
  - name: logs
    dsn: "clickhouse://logs-host:9000"
    database: logs
`)

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, 5*time.Minute, cfg.Interval)
	assert.Equal(t, 9090, cfg.MetricsPort)
	assert.Len(t, cfg.Clusters, 2)
	assert.Equal(t, "default", cfg.Clusters[0].Name)
	assert.Equal(t, "logs", cfg.Clusters[1].Name)
}

func TestUnitLoad_Defaults(t *testing.T) {
	path := writeTemp(t, `
clusters:
  - name: default
    dsn: "clickhouse://localhost:9000"
    database: mydb
`)

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, 3*time.Minute, cfg.Interval)
	assert.Equal(t, 9363, cfg.MetricsPort)
}

func TestUnitLoad_EnvExpansion(t *testing.T) {
	t.Setenv("TEST_CH_DSN", "clickhouse://secret-host:9000")

	path := writeTemp(t, `
clusters:
  - name: default
    dsn: "${TEST_CH_DSN}"
    database: mydb
`)

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, "clickhouse://secret-host:9000", cfg.Clusters[0].DSN)
}

func TestUnitLoad_NoClusters(t *testing.T) {
	path := writeTemp(t, `
clusters: []
`)

	_, err := Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one cluster")
}

func TestUnitLoad_MissingName(t *testing.T) {
	path := writeTemp(t, `
clusters:
  - dsn: "clickhouse://localhost:9000"
    database: mydb
`)

	_, err := Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestUnitLoad_MissingDSN(t *testing.T) {
	path := writeTemp(t, `
clusters:
  - name: default
    database: mydb
`)

	_, err := Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dsn is required")
}

func TestUnitLoad_MissingDatabase(t *testing.T) {
	path := writeTemp(t, `
clusters:
  - name: default
    dsn: "clickhouse://localhost:9000"
`)

	_, err := Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database is required")
}

func TestUnitLoad_InvalidClusterName(t *testing.T) {
	path := writeTemp(t, `
clusters:
  - name: "default'); DROP TABLE foo; --"
    dsn: "clickhouse://localhost:9000"
    database: mydb
`)

	_, err := Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid characters")
}

func TestUnitLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading config file")
}
