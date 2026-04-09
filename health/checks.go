package health

import (
	"context"
	"fmt"
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
	// so we use fmt.Sprintf for the cluster name in these queries.
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

	type hostRow struct {
		Host string `ch:"host"`
	}

	var respondingHosts []hostRow
	probeQuery := fmt.Sprintf(queryProbeNodes, cluster.ClusterName)
	if err := cluster.Conn.Select(queryCtxProbe, &respondingHosts, probeQuery); err != nil {
		c.logger.Error("failed to probe cluster nodes via system.one", "error", err, "cluster", cluster.ClusterName)
		c.metrics.healthCheckErrors.WithLabelValues(cluster.ClusterName, checkNodeReachabilityProbe).Inc()
		return
	}

	respondingSet := make(map[string]bool, len(respondingHosts))
	for _, h := range respondingHosts {
		respondingSet[StripPodOrdinal(ShortHostname(h.Host))] = true
	}

	for _, node := range expectedNodes {
		shardStr := strconv.FormatUint(uint64(node.ShardNum), 10)
		replicaStr := strconv.FormatUint(uint64(node.ReplicaNum), 10)
		if respondingSet[ShortHostname(node.HostName)] {
			c.metrics.nodeReachable.WithLabelValues(cluster.ClusterName, node.HostName, shardStr, replicaStr).Set(1)
			continue
		}

		c.metrics.nodeReachable.WithLabelValues(cluster.ClusterName, node.HostName, shardStr, replicaStr).Set(0)
		c.logger.Warn("ClickHouse node unreachable",
			"cluster", cluster.ClusterName,
			"host", node.HostName,
			"shard", shardStr,
			"replica", replicaStr,
		)
	}
}

// checkReplicaHealth queries replica health metrics across all shards in the cluster.
func (c *Checker) checkReplicaHealth(ctx context.Context, cluster ClusterConfig) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var rows []replicaHealthRow
	query := fmt.Sprintf(queryReplicaHealth, cluster.ClusterName)

	if err := cluster.Conn.Select(queryCtx, &rows, query, clickhouse.Named("database", cluster.Database)); err != nil {
		c.logger.Error("failed to query system.replicas", "error", err, "cluster", cluster.ClusterName)
		c.metrics.healthCheckErrors.WithLabelValues(cluster.ClusterName, checkReplicaHealth).Inc()
		return
	}

	for _, r := range rows {
		labels := []string{cluster.ClusterName, r.Host, r.Database, r.Table, r.ReplicaName}
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
	query := fmt.Sprintf(queryStuckReplicationQueue, cluster.ClusterName)

	if err := cluster.Conn.Select(queryCtx, &rows, query, clickhouse.Named("database", cluster.Database)); err != nil {
		c.logger.Error("failed to query system.replication_queue", "error", err, "cluster", cluster.ClusterName)
		c.metrics.healthCheckErrors.WithLabelValues(cluster.ClusterName, checkStuckReplicationQueue).Inc()
		return
	}

	for _, r := range rows {
		c.metrics.replicationQueueStuckEntries.WithLabelValues(
			cluster.ClusterName, r.Host, r.Database, r.Table, r.Type,
		).Set(float64(r.Cnt))
	}
}

// checkStuckMutations detects incomplete mutations (is_done = 0).
func (c *Checker) checkStuckMutations(ctx context.Context, cluster ClusterConfig) {
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var rows []stuckMutationRow
	query := fmt.Sprintf(queryStuckMutations, cluster.ClusterName)

	if err := cluster.Conn.Select(queryCtx, &rows, query, clickhouse.Named("database", cluster.Database)); err != nil {
		c.logger.Error("failed to query system.mutations", "error", err, "cluster", cluster.ClusterName)
		c.metrics.healthCheckErrors.WithLabelValues(cluster.ClusterName, checkStuckMutations).Inc()
		return
	}

	for _, r := range rows {
		c.metrics.stuckMutations.WithLabelValues(cluster.ClusterName, r.Host, r.Database, r.Table).Set(float64(r.Cnt))
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

	for _, r := range rows {
		c.metrics.ddlQueueStatus.WithLabelValues(cluster.ClusterName, r.Status).Set(float64(r.Cnt))
	}
}

// ShortHostname returns the first segment of a hostname, stripping any domain suffix.
func ShortHostname(host string) string {
	return strings.SplitN(host, ".", 2)[0]
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
