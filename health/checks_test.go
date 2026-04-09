package health

import (
	"context"
	"fmt"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	mockhouse "github.com/srikanthccv/ClickHouse-go-mock"
	"github.com/stretchr/testify/assert"
)

func newTestChecker(t *testing.T, clusterName, database string) (mockhouse.ClickConnMockCommon, ClusterConfig, *Checker) {
	t.Helper()
	conn, cluster := newTestCluster(t, clusterName, database)
	checker := NewChecker([]ClusterConfig{cluster}, WithRegisterer(prometheus.NewRegistry()))
	return conn, cluster, checker
}

func TestUnitCheckNodeReachability_AllNodesUp(t *testing.T) {
	conn, cluster, checker := newTestChecker(t, "default", "siden")

	clusterCols := []mockhouse.ColumnType{
		{Name: "host_name", Type: "String"},
		{Name: "shard_num", Type: "UInt32"},
		{Name: "replica_num", Type: "UInt32"},
	}
	conn.ExpectSelect("SELECT host_name.*FROM system.clusters.*").
		WillReturnRows(mockhouse.NewRows(clusterCols, [][]any{
			{"chi-clickhouse-default-0-0", uint32(1), uint32(1)},
			{"chi-clickhouse-default-0-1", uint32(1), uint32(2)},
			{"chi-clickhouse-default-1-0", uint32(2), uint32(1)},
		}))

	probeCols := []mockhouse.ColumnType{{Name: "host", Type: "String"}}
	conn.ExpectSelect("SELECT hostName.*FROM clusterAllReplicas.*system.one.*").
		WillReturnRows(mockhouse.NewRows(probeCols, [][]any{
			{"chi-clickhouse-default-0-0-0"},
			{"chi-clickhouse-default-0-1-0"},
			{"chi-clickhouse-default-1-0-0"},
		}))

	checker.checkNodeReachability(context.Background(), cluster)
	assert.NoError(t, conn.ExpectationsWereMet())
}

func TestUnitCheckNodeReachability_OneNodeDown(t *testing.T) {
	conn, cluster, checker := newTestChecker(t, "default", "siden")

	clusterCols := []mockhouse.ColumnType{
		{Name: "host_name", Type: "String"},
		{Name: "shard_num", Type: "UInt32"},
		{Name: "replica_num", Type: "UInt32"},
	}
	conn.ExpectSelect("SELECT host_name.*FROM system.clusters.*").
		WillReturnRows(mockhouse.NewRows(clusterCols, [][]any{
			{"node-0-0", uint32(1), uint32(1)},
			{"node-0-1", uint32(1), uint32(2)},
			{"node-1-0", uint32(2), uint32(1)},
		}))

	probeCols := []mockhouse.ColumnType{{Name: "host", Type: "String"}}
	conn.ExpectSelect("SELECT hostName.*FROM clusterAllReplicas.*system.one.*").
		WillReturnRows(mockhouse.NewRows(probeCols, [][]any{
			{"node-0-0-0"},
			{"node-1-0-0"},
		}))

	checker.checkNodeReachability(context.Background(), cluster)
	assert.NoError(t, conn.ExpectationsWereMet())
}

func TestUnitCheckNodeReachability_QueryError(t *testing.T) {
	conn, cluster, checker := newTestChecker(t, "default", "siden")

	conn.ExpectSelect("SELECT host_name.*FROM system.clusters.*").
		WillReturnError(fmt.Errorf("connection refused"))

	checker.checkNodeReachability(context.Background(), cluster)
	assert.NoError(t, conn.ExpectationsWereMet())
}

func TestUnitCheckReplicaHealth_Healthy(t *testing.T) {
	conn, cluster, checker := newTestChecker(t, "default", "siden")

	cols := []mockhouse.ColumnType{
		{Name: "host", Type: "String"}, {Name: "database", Type: "String"},
		{Name: "table", Type: "String"}, {Name: "replica_name", Type: "String"},
		{Name: "is_readonly", Type: "UInt8"},
		{Name: "absolute_delay", Type: "UInt64"}, {Name: "queue_size", Type: "UInt32"},
		{Name: "total_replicas", Type: "UInt32"}, {Name: "active_replicas", Type: "UInt32"},
	}
	conn.ExpectSelect("SELECT hostName.*FROM clusterAllReplicas.*system.replicas.*").
		WillReturnRows(mockhouse.NewRows(cols, [][]any{
			{"node-0-0", "siden", "analytics_request_log", "replica-0-0",
				uint8(0), uint64(0), uint32(0), uint32(3), uint32(3)},
			{"node-0-1", "siden", "analytics_request_log", "replica-0-1",
				uint8(0), uint64(0), uint32(1), uint32(3), uint32(3)},
		}))

	checker.checkReplicaHealth(context.Background(), cluster)
	assert.NoError(t, conn.ExpectationsWereMet())
}

func TestUnitCheckReplicaHealth_HighDelayAndReadonly(t *testing.T) {
	conn, cluster, checker := newTestChecker(t, "default", "siden")

	cols := []mockhouse.ColumnType{
		{Name: "host", Type: "String"}, {Name: "database", Type: "String"},
		{Name: "table", Type: "String"}, {Name: "replica_name", Type: "String"},
		{Name: "is_readonly", Type: "UInt8"},
		{Name: "absolute_delay", Type: "UInt64"}, {Name: "queue_size", Type: "UInt32"},
		{Name: "total_replicas", Type: "UInt32"}, {Name: "active_replicas", Type: "UInt32"},
	}
	conn.ExpectSelect("SELECT hostName.*FROM clusterAllReplicas.*system.replicas.*").
		WillReturnRows(mockhouse.NewRows(cols, [][]any{
			{"node-0-0", "siden", "analytics_request_log", "replica-0-0",
				uint8(1), uint64(5000000), uint32(150), uint32(3), uint32(1)},
		}))

	checker.checkReplicaHealth(context.Background(), cluster)
	assert.NoError(t, conn.ExpectationsWereMet())
}

func TestUnitCheckReplicaHealth_QueryError(t *testing.T) {
	conn, cluster, checker := newTestChecker(t, "default", "siden")

	conn.ExpectSelect("SELECT hostName.*FROM clusterAllReplicas.*system.replicas.*").
		WillReturnError(fmt.Errorf("query timeout"))

	checker.checkReplicaHealth(context.Background(), cluster)
	assert.NoError(t, conn.ExpectationsWereMet())
}

func TestUnitCheckStuckReplicationQueue_NoStuckEntries(t *testing.T) {
	conn, cluster, checker := newTestChecker(t, "default", "siden")

	cols := []mockhouse.ColumnType{
		{Name: "host", Type: "String"}, {Name: "database", Type: "String"},
		{Name: "table", Type: "String"}, {Name: "type", Type: "String"},
		{Name: "cnt", Type: "UInt64"},
	}
	conn.ExpectSelect("SELECT hostName.*FROM clusterAllReplicas.*system.replication_queue.*").
		WillReturnRows(mockhouse.NewRows(cols, [][]any{}))

	checker.checkStuckReplicationQueue(context.Background(), cluster)
	assert.NoError(t, conn.ExpectationsWereMet())
}

func TestUnitCheckStuckReplicationQueue_WithStuckEntries(t *testing.T) {
	conn, cluster, checker := newTestChecker(t, "default", "siden")

	cols := []mockhouse.ColumnType{
		{Name: "host", Type: "String"}, {Name: "database", Type: "String"},
		{Name: "table", Type: "String"}, {Name: "type", Type: "String"},
		{Name: "cnt", Type: "UInt64"},
	}
	conn.ExpectSelect("SELECT hostName.*FROM clusterAllReplicas.*system.replication_queue.*").
		WillReturnRows(mockhouse.NewRows(cols, [][]any{
			{"node-0-0", "siden", "analytics_request_log", "GET_PART", uint64(5)},
			{"node-0-0", "siden", "analytics_request_log", "ALTER_METADATA", uint64(2)},
		}))

	checker.checkStuckReplicationQueue(context.Background(), cluster)
	assert.NoError(t, conn.ExpectationsWereMet())
}

func TestUnitCheckStuckMutations_NoStuckMutations(t *testing.T) {
	conn, cluster, checker := newTestChecker(t, "default", "siden")

	cols := []mockhouse.ColumnType{
		{Name: "host", Type: "String"}, {Name: "database", Type: "String"},
		{Name: "table", Type: "String"}, {Name: "cnt", Type: "UInt64"},
	}
	conn.ExpectSelect("SELECT hostName.*FROM clusterAllReplicas.*system.mutations.*").
		WillReturnRows(mockhouse.NewRows(cols, [][]any{}))

	checker.checkStuckMutations(context.Background(), cluster)
	assert.NoError(t, conn.ExpectationsWereMet())
}

func TestUnitCheckStuckMutations_WithStuckMutations(t *testing.T) {
	conn, cluster, checker := newTestChecker(t, "default", "siden")

	cols := []mockhouse.ColumnType{
		{Name: "host", Type: "String"}, {Name: "database", Type: "String"},
		{Name: "table", Type: "String"}, {Name: "cnt", Type: "UInt64"},
	}
	conn.ExpectSelect("SELECT hostName.*FROM clusterAllReplicas.*system.mutations.*").
		WillReturnRows(mockhouse.NewRows(cols, [][]any{
			{"node-0-0", "siden", "analytics_request_log", uint64(3)},
		}))

	checker.checkStuckMutations(context.Background(), cluster)
	assert.NoError(t, conn.ExpectationsWereMet())
}

func TestUnitCheckDDLQueueStatus_AllFinished(t *testing.T) {
	conn, cluster, checker := newTestChecker(t, "default", "siden")

	cols := []mockhouse.ColumnType{
		{Name: "status", Type: "String"}, {Name: "cnt", Type: "UInt64"},
	}
	conn.ExpectSelect("SELECT status.*FROM system.distributed_ddl_queue.*").
		WillReturnRows(mockhouse.NewRows(cols, [][]any{{"Finished", uint64(42)}}))

	checker.checkDDLQueueStatus(context.Background(), cluster)
	assert.NoError(t, conn.ExpectationsWereMet())
}

func TestUnitCheckDDLQueueStatus_InactiveEntries(t *testing.T) {
	conn, cluster, checker := newTestChecker(t, "default", "siden")

	cols := []mockhouse.ColumnType{
		{Name: "status", Type: "String"}, {Name: "cnt", Type: "UInt64"},
	}
	conn.ExpectSelect("SELECT status.*FROM system.distributed_ddl_queue.*").
		WillReturnRows(mockhouse.NewRows(cols, [][]any{
			{"Finished", uint64(40)},
			{"Inactive", uint64(3)},
		}))

	checker.checkDDLQueueStatus(context.Background(), cluster)
	assert.NoError(t, conn.ExpectationsWereMet())
}

func TestUnitCheckDDLQueueStatus_QueryError(t *testing.T) {
	conn, cluster, checker := newTestChecker(t, "default", "siden")

	conn.ExpectSelect("SELECT status.*FROM system.distributed_ddl_queue.*").
		WillReturnError(fmt.Errorf("connection error"))

	checker.checkDDLQueueStatus(context.Background(), cluster)
	assert.NoError(t, conn.ExpectationsWereMet())
}

func TestUnitStripPodOrdinal(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"chi-clickhouse-default-0-0-0", "chi-clickhouse-default-0-0"},
		{"chi-clickhouse-default-2-1-0", "chi-clickhouse-default-2-1"},
		{"node-0-0-0", "node-0-0"},
		{"node-no-ordinal", "node-no-ordinal"},
		{"single", "single"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, StripPodOrdinal(tt.input))
		})
	}
}

func TestUnitShortHostname(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"chi-clickhouse-default-0-0-0.chi-clickhouse-default-0-0.data.svc.cluster.local", "chi-clickhouse-default-0-0-0"},
		{"node-0-0", "node-0-0"},
		{"simple", "simple"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, ShortHostname(tt.input))
		})
	}
}
