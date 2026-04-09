package health

import "github.com/ClickHouse/clickhouse-go/v2"

// ClusterConfig holds the connection and metadata for a single ClickHouse cluster.
type ClusterConfig struct {
	Conn        clickhouse.Conn
	ClusterName string
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
