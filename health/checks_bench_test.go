package health

import (
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func newBenchChecker(b *testing.B) *Checker {
	b.Helper()
	// Use a discard logger to avoid benchmark noise from warning/error logs.
	discardLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewChecker(nil, WithRegisterer(prometheus.NewRegistry()), WithLogger(discardLogger))
}

// BenchmarkShortHostname benchmarks hostname extraction from a realistic k8s FQDN.
func BenchmarkShortHostname(b *testing.B) {
	host := "chi-clickhouse-default-0-0-0.chi-clickhouse-default-0-0.data.svc.cluster.local"
	b.ReportAllocs()
	for b.Loop() {
		ShortHostname(host)
	}
}

// BenchmarkShortHostname_NoDot benchmarks the no-dot fast path.
func BenchmarkShortHostname_NoDot(b *testing.B) {
	host := "chi-clickhouse-default-0-0-0"
	b.ReportAllocs()
	for b.Loop() {
		ShortHostname(host)
	}
}

// BenchmarkStripPodOrdinal benchmarks pod ordinal stripping from a StatefulSet pod name.
func BenchmarkStripPodOrdinal(b *testing.B) {
	host := "chi-clickhouse-default-0-0-0"
	b.ReportAllocs()
	for b.Loop() {
		StripPodOrdinal(host)
	}
}

// BenchmarkStripPodOrdinal_NoOrdinal benchmarks the no-ordinal fast path.
func BenchmarkStripPodOrdinal_NoOrdinal(b *testing.B) {
	host := "node-no-ordinal"
	b.ReportAllocs()
	for b.Loop() {
		StripPodOrdinal(host)
	}
}

func generateReplicaHealthRows(n int) []replicaHealthRow {
	rows := make([]replicaHealthRow, n)
	for i := range rows {
		rows[i] = replicaHealthRow{
			Host:           fmt.Sprintf("chi-clickhouse-default-0-%d-0", i),
			Database:       "production",
			Table:          fmt.Sprintf("events_%d", i%10),
			ReplicaName:    fmt.Sprintf("chi-clickhouse-default-0-%d-0", i),
			IsReadonly:     0,
			AbsoluteDelay:  0,
			QueueSize:      0,
			TotalReplicas:  3,
			ActiveReplicas: 3,
		}
	}
	return rows
}

// BenchmarkSetReplicaHealthMetrics benchmarks metric-setting for replica health with 50 rows.
func BenchmarkSetReplicaHealthMetrics(b *testing.B) {
	c := newBenchChecker(b)
	rows := generateReplicaHealthRows(50)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c.setReplicaHealthMetrics("default", rows)
	}
}

// BenchmarkSetNodeReachabilityMetrics benchmarks metric-setting for node reachability.
func BenchmarkSetNodeReachabilityMetrics(b *testing.B) {
	c := newBenchChecker(b)
	expected := make([]clusterNode, 9)
	for i := range expected {
		expected[i] = clusterNode{
			HostName:   fmt.Sprintf("chi-clickhouse-default-0-%d", i),
			ShardNum:   uint32(i/3 + 1),
			ReplicaNum: uint32(i%3 + 1),
		}
	}
	// 8 of 9 nodes are responding.
	responding := make([]hostRow, 8)
	for i := range responding {
		responding[i] = hostRow{
			Host: fmt.Sprintf("chi-clickhouse-default-0-%d-0.chi-clickhouse-default.data.svc.cluster.local", i),
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c.setNodeReachabilityMetrics("default", expected, responding)
	}
}

// BenchmarkSetStuckReplicationQueueMetrics benchmarks metric-setting for stuck queue entries.
func BenchmarkSetStuckReplicationQueueMetrics(b *testing.B) {
	c := newBenchChecker(b)
	rows := make([]stuckQueueRow, 10)
	for i := range rows {
		rows[i] = stuckQueueRow{
			Host:     fmt.Sprintf("chi-clickhouse-default-0-%d-0", i),
			Database: "production",
			Table:    fmt.Sprintf("events_%d", i),
			Type:     "GET_PART",
			Cnt:      uint64(i + 1),
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c.setStuckReplicationQueueMetrics("default", rows)
	}
}

// BenchmarkSetStuckMutationsMetrics benchmarks metric-setting for stuck mutations.
func BenchmarkSetStuckMutationsMetrics(b *testing.B) {
	c := newBenchChecker(b)
	rows := make([]stuckMutationRow, 5)
	for i := range rows {
		rows[i] = stuckMutationRow{
			Host:     fmt.Sprintf("chi-clickhouse-default-0-%d-0", i),
			Database: "production",
			Table:    fmt.Sprintf("events_%d", i),
			Cnt:      uint64(i + 1),
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c.setStuckMutationsMetrics("default", rows)
	}
}

// BenchmarkSetDDLQueueStatusMetrics benchmarks metric-setting for DDL queue status.
func BenchmarkSetDDLQueueStatusMetrics(b *testing.B) {
	c := newBenchChecker(b)
	rows := []ddlQueueRow{
		{Status: "Finished", Cnt: 42},
		{Status: "Active", Cnt: 3},
		{Status: "Inactive", Cnt: 1},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c.setDDLQueueStatusMetrics("default", rows)
	}
}
