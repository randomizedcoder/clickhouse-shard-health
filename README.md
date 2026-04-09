# clickhouse-shard-health

Lightweight sidecar that monitors ClickHouse cluster health and exposes Prometheus metrics. Designed for [clickhouse-operator](https://github.com/Altinity/clickhouse-operator) deployments on Kubernetes, but works with any ClickHouse cluster.

## What it monitors

| Check | Source table | What it detects |
|---|---|---|
| Node reachability | `system.clusters` + `clusterAllReplicas(cluster, system.one)` | Nodes that are expected but not responding |
| Replica health | `system.replicas` | Replication delay, queue size, readonly replicas, active vs total replica count |
| Stuck replication queue | `system.replication_queue` | Entries with more than 10 retries |
| Stuck mutations | `system.mutations` | Mutations that have not completed (`is_done = 0`) |
| DDL queue status | `system.distributed_ddl_queue` | DDL entries grouped by status (Finished, Inactive, etc.) |

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
| `health_check_errors` | Counter | cluster, query | Errors encountered during health check queries |

## Quick start

### Prerequisites

- Go 1.24+
- A ClickHouse cluster with a monitoring user (see [ClickHouse grants](#clickhouse-grants))

### Build

```bash
make build
```

The binary is written to `bin/clickhouse-shard-health`.

### Configure

Copy the example config and edit it:

```bash
cp config.example.yaml config.yaml
```

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
| `metricsPort` | `9363` | Port for the Prometheus `/metrics` endpoint |
| `clusters[].name` | required | Cluster name as it appears in `system.clusters` |
| `clusters[].dsn` | required | ClickHouse connection string |
| `clusters[].database` | required | Database to monitor for replica health checks |

### Run

```bash
# Default config path: config.yaml
./bin/clickhouse-shard-health

# Custom config path
CONFIG_PATH=/etc/clickhouse-shard-health/config.yaml ./bin/clickhouse-shard-health
```

### Docker

```bash
make docker
docker run -v $(pwd)/config.yaml:/config.yaml -p 9363:9363 siden-io/clickhouse-shard-health
```

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

## Local development setup

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

Verify metrics are exposed:

```bash
curl -s localhost:9363/metrics | grep clickhouse_shard_health
```

### 3. Run Prometheus

Create a `prometheus.yml` scrape config:

```bash
cat > prometheus.yml <<'EOF'
global:
  scrape_interval: 15s
scrape_configs:
  - job_name: clickhouse-shard-health
    static_configs:
      - targets: ["host.docker.internal:9363"]
EOF
```

Start Prometheus:

```bash
docker run -d --name prometheus \
  -p 9090:9090 \
  -v $(pwd)/prometheus.yml:/etc/prometheus/prometheus.yml \
  prom/prometheus
```

Verify the target is UP at http://localhost:9090/targets.

### 4. Run Grafana with auto-provisioned datasource and dashboard

Create provisioning configs:

```bash
mkdir -p grafana/provisioning/datasources grafana/provisioning/dashboards
```

```bash
cat > grafana/provisioning/datasources/prometheus.yml <<'EOF'
apiVersion: 1
datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://host.docker.internal:9090
    isDefault: true
EOF
```

```bash
cat > grafana/provisioning/dashboards/default.yml <<'EOF'
apiVersion: 1
providers:
  - name: default
    folder: ClickHouse
    type: file
    options:
      path: /var/lib/grafana/dashboards
EOF
```

Start Grafana, mounting provisioning configs and the dashboard JSON:

```bash
docker run -d --name grafana \
  -p 3000:3000 \
  -v $(pwd)/grafana/provisioning:/etc/grafana/provisioning \
  -v $(pwd)/dashboards:/var/lib/grafana/dashboards \
  -e GF_AUTH_ANONYMOUS_ENABLED=true \
  -e GF_AUTH_ANONYMOUS_ORG_ROLE=Admin \
  grafana/grafana
```

Open http://localhost:3000, navigate to **Dashboards > ClickHouse > ClickHouse Shard Health**. The `ds` template variable auto-selects the provisioned Prometheus datasource.

### Cleanup

```bash
docker rm -f prometheus grafana
rm -rf grafana/ prometheus.yml
```

## Tests

```bash
make test
```

## License

[MIT](LICENSE)
