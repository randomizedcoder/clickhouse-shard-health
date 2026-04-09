-- Required ClickHouse grants for shard health monitoring.
-- Replace <username> with the user that clickhouse-shard-health connects as.
-- Replace <cluster_name> with your cluster name (e.g., 'default').

-- System table access
GRANT ON CLUSTER '<cluster_name>' SELECT ON system.clusters TO '<username>';
GRANT ON CLUSTER '<cluster_name>' SELECT ON system.replicas TO '<username>';
GRANT ON CLUSTER '<cluster_name>' SELECT ON system.replication_queue TO '<username>';
GRANT ON CLUSTER '<cluster_name>' SELECT ON system.mutations TO '<username>';
GRANT ON CLUSTER '<cluster_name>' SELECT ON system.distributed_ddl_queue TO '<username>';
GRANT ON CLUSTER '<cluster_name>' SELECT ON system.one TO '<username>';

-- Required for clusterAllReplicas() distributed queries
GRANT ON CLUSTER '<cluster_name>' REMOTE ON *.* TO '<username>';
