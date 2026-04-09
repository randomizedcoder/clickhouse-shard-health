package health

import "github.com/prometheus/client_golang/prometheus"

// metrics holds all Prometheus collectors for a Checker instance.
type metrics struct {
	nodeReachable                *prometheus.GaugeVec
	replicaAbsoluteDelay         *prometheus.GaugeVec
	replicaQueueSize             *prometheus.GaugeVec
	replicaIsReadonly            *prometheus.GaugeVec
	replicaActiveCount           *prometheus.GaugeVec
	replicaTotalCount            *prometheus.GaugeVec
	replicationQueueStuckEntries *prometheus.GaugeVec
	stuckMutations               *prometheus.GaugeVec
	ddlQueueStatus               *prometheus.GaugeVec
	healthCheckErrors            *prometheus.CounterVec
}

func newMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		nodeReachable: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "clickhouse_shard_health",
				Name:      "node_reachable",
				Help:      "Whether a ClickHouse node is reachable (1=up, 0=down)",
			},
			[]string{"cluster", "host", "shard_num", "replica_num"},
		),
		replicaAbsoluteDelay: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "clickhouse_shard_health",
				Name:      "replica_absolute_delay",
				Help:      "Replication absolute delay (0 = healthy)",
			},
			[]string{"cluster", "host", "database", "table", "replica_name"},
		),
		replicaQueueSize: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "clickhouse_shard_health",
				Name:      "replica_queue_size",
				Help:      "Replication queue size",
			},
			[]string{"cluster", "host", "database", "table", "replica_name"},
		),
		replicaIsReadonly: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "clickhouse_shard_health",
				Name:      "replica_is_readonly",
				Help:      "Whether a replica is in readonly mode (1=readonly)",
			},
			[]string{"cluster", "host", "database", "table", "replica_name"},
		),
		replicaActiveCount: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "clickhouse_shard_health",
				Name:      "replica_active_count",
				Help:      "Number of active replicas for a table",
			},
			[]string{"cluster", "host", "database", "table", "replica_name"},
		),
		replicaTotalCount: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "clickhouse_shard_health",
				Name:      "replica_total_count",
				Help:      "Total number of replicas for a table",
			},
			[]string{"cluster", "host", "database", "table", "replica_name"},
		),
		replicationQueueStuckEntries: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "clickhouse_shard_health",
				Name:      "replication_queue_stuck_entries",
				Help:      "Count of stuck replication queue entries (num_tries > 10)",
			},
			[]string{"cluster", "host", "database", "table", "type"},
		),
		stuckMutations: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "clickhouse_shard_health",
				Name:      "stuck_mutations",
				Help:      "Count of stuck mutations (is_done = 0)",
			},
			[]string{"cluster", "host", "database", "table"},
		),
		ddlQueueStatus: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "clickhouse_shard_health",
				Name:      "ddl_queue_status",
				Help:      "DDL queue entry count by status",
			},
			[]string{"cluster", "status"},
		),
		healthCheckErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "clickhouse_shard_health",
				Name:      "health_check_errors",
				Help:      "Errors encountered during health check queries",
			},
			[]string{"cluster", "query"},
		),
	}

	reg.MustRegister(
		m.nodeReachable,
		m.replicaAbsoluteDelay,
		m.replicaQueueSize,
		m.replicaIsReadonly,
		m.replicaActiveCount,
		m.replicaTotalCount,
		m.replicationQueueStuckEntries,
		m.stuckMutations,
		m.ddlQueueStatus,
		m.healthCheckErrors,
	)

	return m
}

// resetTransient resets gauge metrics that represent point-in-time state.
// Without this, stale label sets (e.g. dropped tables, resolved mutations)
// would persist indefinitely with their last-seen values.
func (m *metrics) resetTransient() {
	m.nodeReachable.Reset()
	m.replicaAbsoluteDelay.Reset()
	m.replicaQueueSize.Reset()
	m.replicaIsReadonly.Reset()
	m.replicaActiveCount.Reset()
	m.replicaTotalCount.Reset()
	m.replicationQueueStuckEntries.Reset()
	m.stuckMutations.Reset()
	m.ddlQueueStatus.Reset()
}
