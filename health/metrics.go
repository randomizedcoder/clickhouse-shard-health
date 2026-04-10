package health

import "github.com/prometheus/client_golang/prometheus"

const metricsNamespace = "clickhouse_shard_health"

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
	checkDuration                *prometheus.HistogramVec
}

func newGaugeVec(name, help string, labels []string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace, Name: name, Help: help,
	}, labels)
}

func newMetrics(reg prometheus.Registerer) *metrics {
	replicaLabels := []string{"cluster", "host", "database", "table", "replica_name"}

	m := &metrics{
		nodeReachable:                newGaugeVec("node_reachable", "Whether a ClickHouse node is reachable (1=up, 0=down)", []string{"cluster", "host", "shard_num", "replica_num"}),
		replicaAbsoluteDelay:         newGaugeVec("replica_absolute_delay", "Replication absolute delay (0 = healthy)", replicaLabels),
		replicaQueueSize:             newGaugeVec("replica_queue_size", "Replication queue size", replicaLabels),
		replicaIsReadonly:            newGaugeVec("replica_is_readonly", "Whether a replica is in readonly mode (1=readonly)", replicaLabels),
		replicaActiveCount:           newGaugeVec("replica_active_count", "Number of active replicas for a table", replicaLabels),
		replicaTotalCount:            newGaugeVec("replica_total_count", "Total number of replicas for a table", replicaLabels),
		replicationQueueStuckEntries: newGaugeVec("replication_queue_stuck_entries", "Count of stuck replication queue entries (num_tries > 10)", []string{"cluster", "host", "database", "table", "type"}),
		stuckMutations:               newGaugeVec("stuck_mutations", "Count of stuck mutations (is_done = 0)", []string{"cluster", "host", "database", "table"}),
		ddlQueueStatus:               newGaugeVec("ddl_queue_status", "DDL queue entry count by status", []string{"cluster", "status"}),
		healthCheckErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace, Name: "health_check_errors", Help: "Errors encountered during health check queries",
		}, []string{"cluster", "query"}),
		checkDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: metricsNamespace, Name: "check_duration_seconds", Help: "Duration of a complete health check cycle per cluster",
			Buckets: prometheus.DefBuckets,
		}, []string{"cluster"}),
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
		m.checkDuration,
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
