package health

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

const (
	queryTimeout = 10 * time.Second

	// Error label constants for healthCheckErrors metric.
	checkNodeReachabilityExpected = "node_reachability_expected"
	checkNodeReachabilityProbe    = "node_reachability_probe"
	checkReplicaHealth            = "replica_health"
	checkStuckReplicationQueue    = "stuck_replication_queue"
	checkStuckMutations           = "stuck_mutations"
	checkDDLQueueStatus           = "ddl_queue_status"

	queryExpectedNodes = "SELECT host_name, shard_num, replica_num FROM system.clusters WHERE cluster = @cluster"

	// clusterAllReplicas() is a table function — its arguments cannot be parameterized,
	// so the cluster name is interpolated via fmt.Sprintf at startup (see clusterQueries).
	queryProbeNodes = "SELECT hostName() AS host FROM clusterAllReplicas('%s', system.one) " +
		"SETTINGS skip_unavailable_shards = 1"

	queryReplicaHealth = `SELECT hostName() AS host, database, table, replica_name,
		is_readonly, absolute_delay, queue_size,
		total_replicas, active_replicas
	FROM clusterAllReplicas('%s', system.replicas)
	WHERE database = @database
	SETTINGS skip_unavailable_shards = 1`

	queryStuckReplicationQueue = `SELECT hostName() AS host, database, table, type,
		count() AS cnt
	FROM clusterAllReplicas('%s', system.replication_queue)
	WHERE database = @database AND num_tries > 10
	GROUP BY host, database, table, type
	SETTINGS skip_unavailable_shards = 1`

	queryStuckMutations = `SELECT hostName() AS host, database, table, count() AS cnt
	FROM clusterAllReplicas('%s', system.mutations)
	WHERE database = @database AND is_done = 0
	GROUP BY host, database, table
	SETTINGS skip_unavailable_shards = 1`

	queryDDLQueueStatus = `SELECT status, count() AS cnt
	FROM system.distributed_ddl_queue
	WHERE cluster = @cluster
	GROUP BY status`
)

// checkNodeReachability queries system.clusters for expected nodes and probes reachability
// via clusterAllReplicas(). Unreachable nodes are logged and set to 0 in the metric.
func (c *Checker) checkNodeReachability(ctx context.Context, cluster ClusterConfig) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var expectedNodes []clusterNode
	if err := cluster.Conn.Select(queryCtx, &expectedNodes, queryExpectedNodes,
		clickhouse.Named("cluster", cluster.ClusterName)); err != nil {
		c.logger.Error("failed to query system.clusters", "error", err, "cluster", cluster.ClusterName)
		c.metrics.healthCheckErrors.WithLabelValues(cluster.ClusterName, checkNodeReachabilityExpected).Inc()
		return
	}

	queryCtxProbe, cancelProbe := context.WithTimeout(ctx, queryTimeout)
	defer cancelProbe()

	var respondingHosts []hostRow
	if err := cluster.Conn.Select(queryCtxProbe, &respondingHosts, c.queries[cluster.ClusterName].probeNodes); err != nil {
		c.logger.Error("failed to probe cluster nodes via system.one", "error", err, "cluster", cluster.ClusterName)
		c.metrics.healthCheckErrors.WithLabelValues(cluster.ClusterName, checkNodeReachabilityProbe).Inc()
		return
	}

	c.setNodeReachabilityMetrics(cluster.ClusterName, expectedNodes, respondingHosts)
}

// setNodeReachabilityMetrics compares expected nodes against responding hosts and sets metrics.
func (c *Checker) setNodeReachabilityMetrics(clusterName string, expected []clusterNode, responding []hostRow) {
	respondingSet := make(map[string]bool, len(responding))
	for _, h := range responding {
		respondingSet[StripPodOrdinal(ShortHostname(h.Host))] = true
	}

	labels := make([]string, 4)
	labels[0] = clusterName

	for _, node := range expected {
		labels[1] = node.HostName
		labels[2] = strconv.FormatUint(uint64(node.ShardNum), 10)
		labels[3] = strconv.FormatUint(uint64(node.ReplicaNum), 10)

		if respondingSet[ShortHostname(node.HostName)] {
			c.metrics.nodeReachable.WithLabelValues(labels...).Set(1)
			continue
		}

		c.metrics.nodeReachable.WithLabelValues(labels...).Set(0)
		c.logger.Warn("ClickHouse node unreachable",
			"cluster", clusterName,
			"host", node.HostName,
			"shard", labels[2],
			"replica", labels[3],
		)
	}
}

// checkReplicaHealth queries replica health metrics across all shards in the cluster.
func (c *Checker) checkReplicaHealth(ctx context.Context, cluster ClusterConfig) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var rows []replicaHealthRow
	if err := cluster.Conn.Select(queryCtx, &rows, c.queries[cluster.ClusterName].replicaHealth,
		clickhouse.Named("database", cluster.Database)); err != nil {
		c.logger.Error("failed to query system.replicas", "error", err, "cluster", cluster.ClusterName)
		c.metrics.healthCheckErrors.WithLabelValues(cluster.ClusterName, checkReplicaHealth).Inc()
		return
	}

	c.setReplicaHealthMetrics(cluster.ClusterName, rows)
}

// setReplicaHealthMetrics sets Prometheus metrics from replica health query results.
func (c *Checker) setReplicaHealthMetrics(clusterName string, rows []replicaHealthRow) {
	labels := make([]string, 5)
	labels[0] = clusterName

	for _, r := range rows {
		labels[1] = r.Host
		labels[2] = r.Database
		labels[3] = r.Table
		labels[4] = r.ReplicaName
		c.metrics.replicaAbsoluteDelay.WithLabelValues(labels...).Set(float64(r.AbsoluteDelay))
		c.metrics.replicaQueueSize.WithLabelValues(labels...).Set(float64(r.QueueSize))
		c.metrics.replicaIsReadonly.WithLabelValues(labels...).Set(float64(r.IsReadonly))
		c.metrics.replicaActiveCount.WithLabelValues(labels...).Set(float64(r.ActiveReplicas))
		c.metrics.replicaTotalCount.WithLabelValues(labels...).Set(float64(r.TotalReplicas))
	}
}

// checkStuckReplicationQueue detects replication queue entries with num_tries > 10.
func (c *Checker) checkStuckReplicationQueue(ctx context.Context, cluster ClusterConfig) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var rows []stuckQueueRow
	if err := cluster.Conn.Select(queryCtx, &rows, c.queries[cluster.ClusterName].stuckReplicationQueue,
		clickhouse.Named("database", cluster.Database)); err != nil {
		c.logger.Error("failed to query system.replication_queue", "error", err, "cluster", cluster.ClusterName)
		c.metrics.healthCheckErrors.WithLabelValues(cluster.ClusterName, checkStuckReplicationQueue).Inc()
		return
	}

	c.setStuckReplicationQueueMetrics(cluster.ClusterName, rows)
}

// setStuckReplicationQueueMetrics sets Prometheus metrics from stuck replication queue results.
func (c *Checker) setStuckReplicationQueueMetrics(clusterName string, rows []stuckQueueRow) {
	labels := make([]string, 5)
	labels[0] = clusterName

	for _, r := range rows {
		labels[1] = r.Host
		labels[2] = r.Database
		labels[3] = r.Table
		labels[4] = r.Type
		c.metrics.replicationQueueStuckEntries.WithLabelValues(labels...).Set(float64(r.Cnt))
	}
}

// checkStuckMutations detects incomplete mutations (is_done = 0).
func (c *Checker) checkStuckMutations(ctx context.Context, cluster ClusterConfig) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var rows []stuckMutationRow
	if err := cluster.Conn.Select(queryCtx, &rows, c.queries[cluster.ClusterName].stuckMutations,
		clickhouse.Named("database", cluster.Database)); err != nil {
		c.logger.Error("failed to query system.mutations", "error", err, "cluster", cluster.ClusterName)
		c.metrics.healthCheckErrors.WithLabelValues(cluster.ClusterName, checkStuckMutations).Inc()
		return
	}

	c.setStuckMutationsMetrics(cluster.ClusterName, rows)
}

// setStuckMutationsMetrics sets Prometheus metrics from stuck mutation results.
func (c *Checker) setStuckMutationsMetrics(clusterName string, rows []stuckMutationRow) {
	labels := make([]string, 4)
	labels[0] = clusterName

	for _, r := range rows {
		labels[1] = r.Host
		labels[2] = r.Database
		labels[3] = r.Table
		c.metrics.stuckMutations.WithLabelValues(labels...).Set(float64(r.Cnt))
	}
}

// checkDDLQueueStatus queries the distributed DDL queue and groups entries by status.
func (c *Checker) checkDDLQueueStatus(ctx context.Context, cluster ClusterConfig) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var rows []ddlQueueRow
	if err := cluster.Conn.Select(queryCtx, &rows, queryDDLQueueStatus,
		clickhouse.Named("cluster", cluster.ClusterName)); err != nil {
		c.logger.Error("failed to query system.distributed_ddl_queue", "error", err, "cluster", cluster.ClusterName)
		c.metrics.healthCheckErrors.WithLabelValues(cluster.ClusterName, checkDDLQueueStatus).Inc()
		return
	}

	c.setDDLQueueStatusMetrics(cluster.ClusterName, rows)
}

// setDDLQueueStatusMetrics sets Prometheus metrics from DDL queue status results.
func (c *Checker) setDDLQueueStatusMetrics(clusterName string, rows []ddlQueueRow) {
	labels := make([]string, 2)
	labels[0] = clusterName

	for _, r := range rows {
		labels[1] = r.Status
		c.metrics.ddlQueueStatus.WithLabelValues(labels...).Set(float64(r.Cnt))
	}
}

// ShortHostname returns the first segment of a hostname, stripping any domain suffix.
func ShortHostname(host string) string {
	if idx := strings.IndexByte(host, '.'); idx >= 0 {
		return host[:idx]
	}
	return host
}

// StripPodOrdinal removes the trailing StatefulSet pod ordinal from a ClickHouse hostname.
// The clickhouse-operator names pods as "{statefulset}-{ordinal}" (e.g. "chi-clickhouse-default-0-0-0"),
// while system.clusters stores just the StatefulSet name (e.g. "chi-clickhouse-default-0-0").
func StripPodOrdinal(host string) string {
	idx := strings.LastIndex(host, "-")
	if idx < 0 {
		return host
	}
	if _, err := strconv.Atoi(host[idx+1:]); err == nil {
		return host[:idx]
	}
	return host
}
