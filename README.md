# clickhouse-shard-health

Lightweight sidecar that monitors ClickHouse cluster health and exposes Prometheus metrics.

## Why this tool

ClickHouse replication problems are silent killers. A node goes down, replication lag creeps up, a mutation gets stuck — and nothing alerts you until a query fails or data diverges across replicas. The built-in system tables have all the information you need, but nobody is watching them.

**clickhouse-shard-health** continuously queries ClickHouse system tables across every shard and replica in your cluster, detects problems early, and exposes everything as Prometheus metrics. Pair it with the included Grafana dashboard and you get full visibility into cluster health without writing a single query.

- **Single static binary** — no runtime dependencies, no JVM, no Python
- **Minimal footprint** — runs as a sidecar with negligible CPU and memory overhead
- **Tiny container** — ~6 MB OCI image (binary + CA certs, no shell)
- **Multi-architecture** — native x86_64 and Darwin builds, cross-compiled ARM64 and RISC-V
- **Works anywhere** — designed for [clickhouse-operator](https://github.com/Altinity/clickhouse-operator) on Kubernetes, but works with any ClickHouse cluster
- **Built-in profiling** — pprof endpoints for production debugging

## What it monitors

| Check | Source table | What it detects |
|---|---|---|
| Node reachability | `system.clusters` + `clusterAllReplicas(cluster, system.one)` | Nodes that are expected but not responding |
| Replica health | `system.replicas` | Replication delay, queue size, readonly replicas, active vs total replica count |
| Stuck replication queue | `system.replication_queue` | Entries with more than 10 retries |
| Stuck mutations | `system.mutations` | Mutations that have not completed (`is_done = 0`) |
| DDL queue status | `system.distributed_ddl_queue` | DDL entries grouped by status (Finished, Inactive, etc.) |

## Quick start

### Option A: Using Nix (recommended)

[Nix](https://nixos.org) handles all dependencies automatically — no need to install Go or any other tooling manually. See [nix/readme.md](nix/readme.md) for installation and full details.

```bash
nix develop          # Enter dev shell with Go and all tools
cp config.example.yaml config.yaml
# Edit config.yaml with your cluster details
make build && ./bin/clickhouse-shard-health
```

### Option B: Manual setup

#### Prerequisites

- Go 1.26+
- A ClickHouse cluster with a monitoring user (see [ClickHouse grants](#clickhouse-grants))

#### Build and run

```bash
make build

cp config.example.yaml config.yaml
# Edit config.yaml with your cluster details

./bin/clickhouse-shard-health
```

Verify metrics are being exposed:

```bash
curl -s localhost:9363/metrics | grep clickhouse_shard_health
```

### Docker

#### Using Nix (reproducible OCI image, ~6 MB)

```bash
nix build .#container
docker load < ./result
docker run --rm -v $(pwd)/config.yaml:/config.yaml -p 9363:9363 clickhouse-shard-health:<version>
```

See [nix/readme.md](nix/readme.md) for all container variants (stripped, debug) and cross-compiled images (ARM64, RISC-V).

#### Using make

```bash
make docker
docker run -v $(pwd)/config.yaml:/config.yaml -p 9363:9363 mmtretiak/clickhouse-shard-health
```

## Configuration

```yaml
interval: 3m
metricsPort: 9363

clusters:
  - name: default
    dsn: "clickhouse://readonly:password@clickhouse-host:9000/mydb?secure=false"
    database: mydb
```

Environment variables are expanded in all string values (`${CLICKHOUSE_PASSWORD}`).

| Field | Default | Description |
|---|---|---|
| `interval` | `3m` | How often to run health checks |
| `metricsPort` | `9363` | Port for the HTTP server (metrics, health, pprof) |
| `clusters[].name` | required | Cluster name as it appears in `system.clusters` |
| `clusters[].dsn` | required | ClickHouse connection string |
| `clusters[].database` | required | Database to monitor for replica health checks |

Set a custom config path with the `CONFIG_PATH` environment variable (default: `config.yaml`).

### CLI flags and environment variables

| Flag | Environment Variable | Default | Description |
|---|---|---|---|
| `-pprof` | `PPROF_ENABLED` | disabled | Enable pprof profiling endpoints at `/debug/pprof/` |

The environment variable takes precedence — if `PPROF_ENABLED` is set to any non-empty value, pprof is enabled regardless of the flag.

## Metrics

All metrics are prefixed with `clickhouse_shard_health_`.

| Metric | Type | Labels | Description |
|---|---|---|---|
| `node_reachable` | Gauge | cluster, host, shard_num, replica_num | 1 if the node is reachable, 0 if down |
| `replica_absolute_delay` | Gauge | cluster, host, database, table, replica_name | Replication delay in seconds (0 = healthy) |
| `replica_queue_size` | Gauge | cluster, host, database, table, replica_name | Number of entries in the replication queue |
| `replica_is_readonly` | Gauge | cluster, host, database, table, replica_name | 1 if the replica is in readonly mode |
| `replica_active_count` | Gauge | cluster, host, database, table, replica_name | Number of active replicas for a table |
| `replica_total_count` | Gauge | cluster, host, database, table, replica_name | Total number of replicas for a table |
| `replication_queue_stuck_entries` | Gauge | cluster, host, database, table, type | Count of stuck replication queue entries |
| `stuck_mutations` | Gauge | cluster, host, database, table | Count of incomplete mutations |
| `ddl_queue_status` | Gauge | cluster, status | DDL queue entry count by status |
| `check_duration_seconds` | Histogram | cluster | Duration of a complete health check cycle per cluster |
| `health_check_errors` | Counter | cluster, query | Errors encountered during health check queries |

## ClickHouse grants

The monitoring user needs read access to several system tables and the `REMOTE` privilege for `clusterAllReplicas()` queries. Apply the grants in [`sql/grants.sql`](sql/grants.sql):

```sql
GRANT ON CLUSTER '<cluster_name>' SELECT ON system.clusters TO '<username>';
GRANT ON CLUSTER '<cluster_name>' SELECT ON system.replicas TO '<username>';
GRANT ON CLUSTER '<cluster_name>' SELECT ON system.replication_queue TO '<username>';
GRANT ON CLUSTER '<cluster_name>' SELECT ON system.mutations TO '<username>';
GRANT ON CLUSTER '<cluster_name>' SELECT ON system.distributed_ddl_queue TO '<username>';
GRANT ON CLUSTER '<cluster_name>' SELECT ON system.one TO '<username>';
GRANT ON CLUSTER '<cluster_name>' REMOTE ON *.* TO '<username>';
```

## Grafana dashboard

A pre-built dashboard is included at [`dashboards/clickhouse-shard-health.json`](dashboards/clickhouse-shard-health.json). Import it into Grafana and select a Prometheus datasource.

The dashboard has panels for:
- Node reachability table (UP/DOWN status per host)
- Absolute replication delay over time
- Queue size over time
- Readonly replicas count
- Active vs total replicas per table
- Stuck replication queue entries
- Stuck mutations count
- DDL queue status breakdown
- Health check query error rate

## Endpoints

The HTTP server (default port `9363`) exposes:

| Path | Description |
|---|---|
| `/metrics` | Prometheus metrics |
| `/healthz` | Liveness probe (returns `200 OK`) |

### Profiling (opt-in)

When enabled with `-pprof` or `PPROF_ENABLED`, the following endpoints are registered:

| Path | Description |
|---|---|
| `/debug/pprof/` | Go pprof index — CPU, memory, goroutine profiling |
| `/debug/pprof/profile` | CPU profile (30s default, `?seconds=N` to customize) |
| `/debug/pprof/heap` | Heap memory profile |
| `/debug/pprof/trace` | Execution trace |

Example: capture a 10-second CPU profile:

```bash
# Start with profiling enabled
./bin/clickhouse-shard-health -pprof
# or: PPROF_ENABLED=1 ./bin/clickhouse-shard-health

# Capture profile
go tool pprof http://localhost:9363/debug/pprof/profile?seconds=10
```

## Development

```bash
make build           # Build binary
make test            # Run tests with race detector
make lint            # Run golangci-lint
make fmt             # Format code
```

### Benchmarks

```bash
go test -bench=. -benchmem ./health/
```

Benchmarks cover all metric-setting functions and hostname utilities. Use `b.ReportAllocs()` in new benchmarks to track allocations.

<details>
<summary><strong>Local development stack (Prometheus + Grafana)</strong></summary>

This setup lets you run the full stack locally: health checker pointed at a port-forwarded ClickHouse cluster, with Prometheus and Grafana for visualization.

### 1. Port-forward ClickHouse

```bash
kubectl port-forward svc/clickhouse 9000:9000 -n <namespace>
```

### 2. Create config and run the health checker

```bash
cat > config.yaml <<'EOF'
interval: 30s
metricsPort: 9363
clusters:
  - name: default
    dsn: "clickhouse://readonly:password@localhost:9000/mydb?secure=false"
    database: mydb
EOF

make build && ./bin/clickhouse-shard-health
```

### 3. Run Prometheus

```bash
cat > prometheus.yml <<'EOF'
global:
  scrape_interval: 15s
scrape_configs:
  - job_name: clickhouse-shard-health
    static_configs:
      - targets: ["host.docker.internal:9363"]
EOF

docker run -d --name prometheus \
  -p 9090:9090 \
  -v $(pwd)/prometheus.yml:/etc/prometheus/prometheus.yml \
  prom/prometheus
```

Verify the target is UP at http://localhost:9090/targets.

### 4. Run Grafana with auto-provisioned datasource and dashboard

```bash
mkdir -p grafana/provisioning/datasources grafana/provisioning/dashboards

cat > grafana/provisioning/datasources/prometheus.yml <<'EOF'
apiVersion: 1
datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://host.docker.internal:9090
    isDefault: true
EOF

cat > grafana/provisioning/dashboards/default.yml <<'EOF'
apiVersion: 1
providers:
  - name: default
    folder: ClickHouse
    type: file
    options:
      path: /var/lib/grafana/dashboards
EOF

docker run -d --name grafana \
  -p 3000:3000 \
  -v $(pwd)/grafana/provisioning:/etc/grafana/provisioning \
  -v $(pwd)/dashboards:/var/lib/grafana/dashboards \
  -e GF_AUTH_ANONYMOUS_ENABLED=true \
  -e GF_AUTH_ANONYMOUS_ORG_ROLE=Admin \
  grafana/grafana
```

Open http://localhost:3000, navigate to **Dashboards > ClickHouse > ClickHouse Shard Health**.

### Cleanup

```bash
docker rm -f prometheus grafana
rm -rf grafana/ prometheus.yml
```

</details>

## Nix

This project includes a Nix flake for reproducible builds, development, and CI. See [nix/readme.md](nix/readme.md) for full details.

```bash
nix develop          # Enter dev shell with all tools
nix build            # Build binary (stripped+UPX, smallest)
nix flake check      # Run all CI checks (vet, gosec, golangci-lint tiers, tests)
nix fmt              # Format Nix files
```

### Binary variants

Every target is available in three variants. The default produces the smallest binary using UPX compression.

| Variant | Package | Container | Binary | Container image |
|---|---|---|---|---|
| **UPX** (default) | `nix build` | `nix build .#container` | ~6 MB | ~6 MB |
| **Stripped** | `nix build .#clickhouse-shard-health-stripped` | `nix build .#container-stripped` | ~18 MB | ~7 MB |
| **Debug** | `nix build .#clickhouse-shard-health-debug` | `nix build .#container-debug` | ~20 MB | ~8 MB |

- **UPX** — stripped with `-s -w` then compressed with [UPX](https://upx.github.io/). Smallest binary, ~15ms decompression at startup. See [Shrink your Go binaries](https://words.filippo.io/shrink-your-go-binaries-with-this-one-weird-trick/) for background.
- **Stripped** — stripped with `-s -w` (removes debug symbols and DWARF tables). No startup overhead.
- **Debug** — unstripped, full debug symbols. Use with `delve` or `gdb` for debugging and profiling.

### Cross-compilation

Cross-compiled binaries and containers for ARM64 and RISC-V are available from `x86_64-linux`, each in all three variants:

```bash
nix build .#cross-aarch64-linux                  # ARM64 (stripped+UPX)
nix build .#cross-aarch64-linux-stripped          # ARM64 (stripped)
nix build .#cross-aarch64-linux-debug             # ARM64 (debug)
nix build .#container-aarch64-linux              # ARM64 container

nix build .#cross-riscv64-linux                  # RISC-V 64 (stripped+UPX)
nix build .#container-riscv64-linux              # RISC-V 64 container
```

See [nix/readme.md](nix/readme.md) for the complete package matrix across all architectures.

## License

[MIT](LICENSE)
