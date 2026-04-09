package health

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
	mockhouse "github.com/srikanthccv/ClickHouse-go-mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCluster(t *testing.T, clusterName, database string) (mockhouse.ClickConnMockCommon, ClusterConfig) {
	t.Helper()
	conn, err := mockhouse.NewClickHouseWithQueryMatcher(nil, sqlmock.QueryMatcherRegexp)
	require.NoError(t, err)
	return conn, ClusterConfig{Conn: conn, ClusterName: clusterName, Database: database}
}

func setupAllQueryExpectations(conn mockhouse.ClickConnMockCommon) {
	clusterCols := []mockhouse.ColumnType{
		{Name: "host_name", Type: "String"},
		{Name: "shard_num", Type: "UInt32"},
		{Name: "replica_num", Type: "UInt32"},
	}
	conn.ExpectSelect("SELECT host_name.*FROM system.clusters.*").
		WillReturnRows(mockhouse.NewRows(clusterCols, [][]any{
			{"node-0-0", uint32(1), uint32(1)},
		}))

	probeCols := []mockhouse.ColumnType{{Name: "host", Type: "String"}}
	conn.ExpectSelect("SELECT hostName.*FROM clusterAllReplicas.*system.one.*").
		WillReturnRows(mockhouse.NewRows(probeCols, [][]any{{"node-0-0-0"}}))

	replicaCols := []mockhouse.ColumnType{
		{Name: "host", Type: "String"}, {Name: "database", Type: "String"},
		{Name: "table", Type: "String"}, {Name: "replica_name", Type: "String"},
		{Name: "is_readonly", Type: "UInt8"},
		{Name: "absolute_delay", Type: "UInt64"}, {Name: "queue_size", Type: "UInt32"},
		{Name: "total_replicas", Type: "UInt32"}, {Name: "active_replicas", Type: "UInt32"},
	}
	conn.ExpectSelect("SELECT hostName.*FROM clusterAllReplicas.*system.replicas.*").
		WillReturnRows(mockhouse.NewRows(replicaCols, [][]any{}))

	queueCols := []mockhouse.ColumnType{
		{Name: "host", Type: "String"}, {Name: "database", Type: "String"},
		{Name: "table", Type: "String"}, {Name: "type", Type: "String"},
		{Name: "cnt", Type: "UInt64"},
	}
	conn.ExpectSelect("SELECT hostName.*FROM clusterAllReplicas.*system.replication_queue.*").
		WillReturnRows(mockhouse.NewRows(queueCols, [][]any{}))

	mutCols := []mockhouse.ColumnType{
		{Name: "host", Type: "String"}, {Name: "database", Type: "String"},
		{Name: "table", Type: "String"}, {Name: "cnt", Type: "UInt64"},
	}
	conn.ExpectSelect("SELECT hostName.*FROM clusterAllReplicas.*system.mutations.*").
		WillReturnRows(mockhouse.NewRows(mutCols, [][]any{}))

	ddlCols := []mockhouse.ColumnType{
		{Name: "status", Type: "String"}, {Name: "cnt", Type: "UInt64"},
	}
	conn.ExpectSelect("SELECT status.*FROM system.distributed_ddl_queue.*").
		WillReturnRows(mockhouse.NewRows(ddlCols, [][]any{{"Finished", uint64(5)}}))
}

func TestUnitNewChecker(t *testing.T) {
	_, cluster := newTestCluster(t, "default", "siden")
	c := NewChecker([]ClusterConfig{cluster}, WithRegisterer(prometheus.NewRegistry()))

	require.NotNil(t, c)
	assert.Len(t, c.clusters, 1)
	assert.Equal(t, "default", c.clusters[0].ClusterName)
}

func TestUnitCheckAll_FullRun(t *testing.T) {
	conn, cluster := newTestCluster(t, "default", "siden")
	c := NewChecker([]ClusterConfig{cluster}, WithRegisterer(prometheus.NewRegistry()))

	setupAllQueryExpectations(conn)

	c.CheckAll(context.Background())
	assert.NoError(t, conn.ExpectationsWereMet())
}

func TestUnitCheckAll_MultipleClusters(t *testing.T) {
	conn1, cluster1 := newTestCluster(t, "default", "siden")
	conn2, cluster2 := newTestCluster(t, "default", "kubernetes")

	c := NewChecker([]ClusterConfig{cluster1, cluster2}, WithRegisterer(prometheus.NewRegistry()))

	setupAllQueryExpectations(conn1)
	setupAllQueryExpectations(conn2)

	c.CheckAll(context.Background())
	assert.NoError(t, conn1.ExpectationsWereMet())
	assert.NoError(t, conn2.ExpectationsWereMet())
}

func TestUnitRun_CancelStops(t *testing.T) {
	conn, cluster := newTestCluster(t, "default", "siden")
	c := NewChecker([]ClusterConfig{cluster}, WithRegisterer(prometheus.NewRegistry()))

	setupAllQueryExpectations(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := c.Run(ctx, 1*time.Hour)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
