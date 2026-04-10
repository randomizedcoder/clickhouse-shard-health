package health

import "github.com/ClickHouse/clickhouse-go/v2"

// clusterName identifies a ClickHouse cluster (as registered in system.clusters).
type clusterName = string

// ClusterConfig holds the connection and metadata for a single ClickHouse cluster.
type ClusterConfig struct {
	Conn        clickhouse.Conn
	ClusterName clusterName
	Database    string
}

// clusterNode represents an expected node from system.clusters.
type clusterNode struct {
	HostName   string `ch:"host_name"`
	ShardNum   uint32 `ch:"shard_num"`
	ReplicaNum uint32 `ch:"replica_num"`
}

// replicaHealthRow represents a row from system.replicas.
type replicaHealthRow struct {
	Host           string `ch:"host"`
	Database       string `ch:"database"`
	Table          string `ch:"table"`
	ReplicaName    string `ch:"replica_name"`
	IsReadonly     uint8  `ch:"is_readonly"`
	AbsoluteDelay  uint64 `ch:"absolute_delay"`
	QueueSize      uint32 `ch:"queue_size"`
	TotalReplicas  uint32 `ch:"total_replicas"`
	ActiveReplicas uint32 `ch:"active_replicas"`
}

// stuckQueueRow represents an aggregated stuck replication queue entry.
type stuckQueueRow struct {
	Host     string `ch:"host"`
	Database string `ch:"database"`
	Table    string `ch:"table"`
	Type     string `ch:"type"`
	Cnt      uint64 `ch:"cnt"`
}

// stuckMutationRow represents an aggregated stuck mutation entry.
type stuckMutationRow struct {
	Host     string `ch:"host"`
	Database string `ch:"database"`
	Table    string `ch:"table"`
	Cnt      uint64 `ch:"cnt"`
}

// ddlQueueRow represents a DDL queue status count.
type ddlQueueRow struct {
	Status string `ch:"status"`
	Cnt    uint64 `ch:"cnt"`
}

// hostRow represents a responding host from a clusterAllReplicas() probe.
type hostRow struct {
	Host string `ch:"host"`
}

// clusterQueries holds pre-formatted query strings for a single cluster.
// Queries using clusterAllReplicas() require the cluster name as a literal,
// so we pre-compute them once at startup to avoid repeated fmt.Sprintf calls.
type clusterQueries struct {
	probeNodes            string
	replicaHealth         string
	stuckReplicationQueue string
	stuckMutations        string
}
