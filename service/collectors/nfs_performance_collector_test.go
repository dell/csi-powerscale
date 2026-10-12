/*
 Copyright © 2025-2026 Dell Inc. or its subsidiaries. All Rights Reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package collectors

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/dell/csm-metrics-common/pkg/naming"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockNFSStatsClient is a mock implementation of NFSStatsClient
type mockNFSStatsClient struct {
	stats *NFSBasicStats
	err   error
}

func (m *mockNFSStatsClient) GetNFSStats(_ context.Context) (*NFSBasicStats, error) {
	return m.stats, m.err
}

// U-NFS-NEW: NewNFSPerformanceCollector creates collector with metrics
func TestNewNFSPerformanceCollector(t *testing.T) {
	reg := prometheus.NewRegistry()
	mockClient := &mockNFSStatsClient{}
	c := NewNFSPerformanceCollector(mockClient, reg, "test-cluster")

	assert.NotNil(t, c)
	assert.Equal(t, "test-cluster", c.clusterName)
	assert.NotNil(t, c.bytesRead)
	assert.NotNil(t, c.bytesWrite)
	assert.NotNil(t, c.latencyV3)
	assert.NotNil(t, c.latencyV4)
}

// U-NFS-NAME: Name returns correct collector name
func TestNFSPerformanceCollector_Name(t *testing.T) {
	reg := prometheus.NewRegistry()
	mockClient := &mockNFSStatsClient{}
	c := NewNFSPerformanceCollector(mockClient, reg, "test-cluster")

	assert.Equal(t, "NFSPerformanceCollector", c.Name())
}

// U-NFS-COLLECT-SUCCESS: Collect successfully updates metrics
func TestNFSPerformanceCollector_Collect_Success(t *testing.T) {
	reg := prometheus.NewRegistry()
	stats := &NFSBasicStats{
		Basic: struct {
			SvcCounters struct {
				BytesRead    float64 `json:"bytes_read"`
				BytesWritten float64 `json:"bytes_written"`
			} `json:"svc_counters"`
		}{
			SvcCounters: struct {
				BytesRead    float64 `json:"bytes_read"`
				BytesWritten float64 `json:"bytes_written"`
			}{
				BytesRead:    1024.0,
				BytesWritten: 2048.0,
			},
		},
		Optime: struct {
			V3TimeBuckets []float64 `json:"v3_time_buckets"`
			V4TimeBuckets []float64 `json:"v4_time_buckets"`
		}{
			V3TimeBuckets: []float64{10, 20, 30},
			V4TimeBuckets: []float64{5, 15, 25},
		},
	}
	mockClient := &mockNFSStatsClient{stats: stats}
	c := NewNFSPerformanceCollector(mockClient, reg, "test-cluster")

	err := c.Collect(context.Background())
	require.NoError(t, err)
}

// U-NFS-COLLECT-NIL: Collect handles nil stats gracefully
func TestNFSPerformanceCollector_Collect_NilStats(t *testing.T) {
	reg := prometheus.NewRegistry()
	mockClient := &mockNFSStatsClient{stats: nil}
	c := NewNFSPerformanceCollector(mockClient, reg, "test-cluster")

	err := c.Collect(context.Background())
	require.NoError(t, err)
}

// U-NFS-COLLECT-ERROR: Collect propagates client errors
func TestNFSPerformanceCollector_Collect_Error(t *testing.T) {
	reg := prometheus.NewRegistry()
	mockClient := &mockNFSStatsClient{err: errors.New("API error")}
	c := NewNFSPerformanceCollector(mockClient, reg, "test-cluster")

	err := c.Collect(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get NFS stats")
}

// U-NFS-COLLECT-ZERO-BUCKETS: Collect handles zero bucket values
func TestNFSPerformanceCollector_Collect_ZeroBuckets(t *testing.T) {
	reg := prometheus.NewRegistry()
	stats := &NFSBasicStats{
		Basic: struct {
			SvcCounters struct {
				BytesRead    float64 `json:"bytes_read"`
				BytesWritten float64 `json:"bytes_written"`
			} `json:"svc_counters"`
		}{
			SvcCounters: struct {
				BytesRead    float64 `json:"bytes_read"`
				BytesWritten float64 `json:"bytes_written"`
			}{
				BytesRead:    0,
				BytesWritten: 0,
			},
		},
		Optime: struct {
			V3TimeBuckets []float64 `json:"v3_time_buckets"`
			V4TimeBuckets []float64 `json:"v4_time_buckets"`
		}{
			V3TimeBuckets: []float64{0, 0, 0},
			V4TimeBuckets: []float64{0, 0, 0},
		},
	}
	mockClient := &mockNFSStatsClient{stats: stats}
	c := NewNFSPerformanceCollector(mockClient, reg, "test-cluster")

	err := c.Collect(context.Background())
	require.NoError(t, err)
}

// U-NFS-COLLECT-EMPTY-BUCKETS: Collect handles empty bucket arrays
func TestNFSPerformanceCollector_Collect_EmptyBuckets(t *testing.T) {
	reg := prometheus.NewRegistry()
	stats := &NFSBasicStats{
		Basic: struct {
			SvcCounters struct {
				BytesRead    float64 `json:"bytes_read"`
				BytesWritten float64 `json:"bytes_written"`
			} `json:"svc_counters"`
		}{
			SvcCounters: struct {
				BytesRead    float64 `json:"bytes_read"`
				BytesWritten float64 `json:"bytes_written"`
			}{
				BytesRead:    512.0,
				BytesWritten: 1024.0,
			},
		},
		Optime: struct {
			V3TimeBuckets []float64 `json:"v3_time_buckets"`
			V4TimeBuckets []float64 `json:"v4_time_buckets"`
		}{
			V3TimeBuckets: []float64{},
			V4TimeBuckets: []float64{},
		},
	}
	mockClient := &mockNFSStatsClient{stats: stats}
	c := NewNFSPerformanceCollector(mockClient, reg, "test-cluster")

	err := c.Collect(context.Background())
	require.NoError(t, err)
}

// U-NFS-CLEANUP: Cleanup removes metrics
func TestNFSPerformanceCollector_Cleanup(t *testing.T) {
	reg := prometheus.NewRegistry()
	mockClient := &mockNFSStatsClient{}
	c := NewNFSPerformanceCollector(mockClient, reg, "test-cluster")

	// Should not panic
	assert.NotPanics(t, func() {
		c.Cleanup()
	})
}

// U-NFS-COLLECT-PARTIAL: Collect with partial data (only basic stats)
func TestNFSPerformanceCollector_Collect_PartialData(t *testing.T) {
	reg := prometheus.NewRegistry()
	stats := &NFSBasicStats{
		Basic: struct {
			SvcCounters struct {
				BytesRead    float64 `json:"bytes_read"`
				BytesWritten float64 `json:"bytes_written"`
			} `json:"svc_counters"`
		}{
			SvcCounters: struct {
				BytesRead    float64 `json:"bytes_read"`
				BytesWritten float64 `json:"bytes_written"`
			}{
				BytesRead:    2048.0,
				BytesWritten: 4096.0,
			},
		},
		Optime: struct {
			V3TimeBuckets []float64 `json:"v3_time_buckets"`
			V4TimeBuckets []float64 `json:"v4_time_buckets"`
		}{
			V3TimeBuckets: nil,
			V4TimeBuckets: nil,
		},
	}
	mockClient := &mockNFSStatsClient{stats: stats}
	c := NewNFSPerformanceCollector(mockClient, reg, "test-cluster")

	err := c.Collect(context.Background())
	require.NoError(t, err)
}

// U-NFS-COLLECT-MISSING-OPTIME-PRESERVES-SNAPSHOT: missing optime fields do
// not reset a previously collected latency snapshot.
func TestNFSPerformanceCollector_Collect_MissingOptimePreservesSnapshot(t *testing.T) {
	reg := prometheus.NewRegistry()
	stats := &NFSBasicStats{}
	stats.Optime.V3TimeBuckets = []float64{5, 0, 0, 0, 0, 0, 0, 0}
	mockClient := &mockNFSStatsClient{stats: stats}
	c := NewNFSPerformanceCollector(mockClient, reg, "test-cluster")

	require.NoError(t, c.Collect(context.Background()))
	first := findHistogram(t, reg, "dell_powerscale_nfs_v3_latency_seconds", "test-cluster")
	require.NotNil(t, first)

	stats.Optime.V3TimeBuckets = nil
	require.NoError(t, c.Collect(context.Background()))
	second := findHistogram(t, reg, "dell_powerscale_nfs_v3_latency_seconds", "test-cluster")
	require.NotNil(t, second)
	assert.Equal(t, first.GetSampleCount(), second.GetSampleCount())
}

// findHistogram scrapes reg for a metric family named name and returns the
// *dto.Histogram for the series matching clusterName, or nil if not found.
func findHistogram(t *testing.T, reg *prometheus.Registry, name, clusterName string) *dto.Histogram {
	t.Helper()
	families, err := reg.Gather()
	require.NoError(t, err)
	for _, mf := range families {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == naming.LabelClusterName && l.GetValue() == clusterName {
					return m.GetHistogram()
				}
			}
		}
	}
	return nil
}

// U-NFS-COLLECT-NO-REPLAY-GROWTH: repeated Collect calls with the same OneFS
// snapshot must not accumulate observations across cycles. This is the fix
// for the reported issue: the previous implementation called
// HistogramVec.Observe once per underlying OneFS operation count on every
// collection cycle, and Prometheus histograms never reset, so _count grew
// without bound as long as the process ran. With the const-histogram
// snapshot approach, two consecutive collections of the identical input
// must report the identical, non-doubled _count/_sum.
func TestNFSPerformanceCollector_Collect_NoReplayGrowthAcrossCycles(t *testing.T) {
	reg := prometheus.NewRegistry()
	stats := &NFSBasicStats{}
	stats.Optime.V3TimeBuckets = []float64{1000, 2000, 0, 0, 0, 0, 0, 0}
	mockClient := &mockNFSStatsClient{stats: stats}
	c := NewNFSPerformanceCollector(mockClient, reg, "test-cluster")

	require.NoError(t, c.Collect(context.Background()))
	first := findHistogram(t, reg, "dell_powerscale_nfs_v3_latency_seconds", "test-cluster")
	require.NotNil(t, first)
	firstCount := first.GetSampleCount()
	assert.Equal(t, uint64(3000), firstCount)

	// Collect again with the exact same snapshot from OneFS.
	require.NoError(t, c.Collect(context.Background()))
	second := findHistogram(t, reg, "dell_powerscale_nfs_v3_latency_seconds", "test-cluster")
	require.NotNil(t, second)
	assert.Equal(t, firstCount, second.GetSampleCount(), "repeated collection of the same snapshot must not double _count")
}

// U-NFS-COLLECT-LARGE-COUNT-PERFORMANCE: a single Collect call must stay
// cheap (O(len(naming.HistogramBuckets))) even when OneFS reports very large
// cumulative operation counts, instead of looping once per operation.
func TestNFSPerformanceCollector_Collect_LargeCountIsCheap(t *testing.T) {
	reg := prometheus.NewRegistry()
	stats := &NFSBasicStats{}
	// A large, realistic cumulative count that would make a replay-based
	// implementation (for j := 0; j < int(count); j++ { Observe(...) })
	// take a very long time and allocate heavily.
	stats.Optime.V3TimeBuckets = []float64{1_000_000_000, 0, 0, 0, 0, 0, 0, 0}
	mockClient := &mockNFSStatsClient{stats: stats}
	c := NewNFSPerformanceCollector(mockClient, reg, "test-cluster")

	start := time.Now()
	require.NoError(t, c.Collect(context.Background()))
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 500*time.Millisecond, "Collect must not scale with the OneFS operation count")

	hist := findHistogram(t, reg, "dell_powerscale_nfs_v3_latency_seconds", "test-cluster")
	require.NotNil(t, hist)
	assert.Equal(t, uint64(1_000_000_000), hist.GetSampleCount())
}

// U-NFS-COLLECT-HISTOGRAM-SHAPE: Collect emits a correctly cumulative const
// histogram (bucket[i] includes all counts at or below bucket[i]'s bound).
func TestNFSPerformanceCollector_Collect_HistogramIsCumulative(t *testing.T) {
	reg := prometheus.NewRegistry()
	stats := &NFSBasicStats{}
	// naming.HistogramBuckets = [0.1, 0.5, 1, 2, 5, 10, 30, 60]
	stats.Optime.V3TimeBuckets = []float64{2, 3, 0, 0, 0, 0, 0, 0}
	mockClient := &mockNFSStatsClient{stats: stats}
	c := NewNFSPerformanceCollector(mockClient, reg, "test-cluster")

	require.NoError(t, c.Collect(context.Background()))
	hist := findHistogram(t, reg, "dell_powerscale_nfs_v3_latency_seconds", "test-cluster")
	require.NotNil(t, hist)
	assert.Equal(t, uint64(5), hist.GetSampleCount())

	bucketCumulative := make(map[float64]uint64)
	for _, b := range hist.GetBucket() {
		bucketCumulative[b.GetUpperBound()] = b.GetCumulativeCount()
	}
	assert.Equal(t, uint64(2), bucketCumulative[0.1])
	assert.Equal(t, uint64(5), bucketCumulative[0.5])
	assert.Equal(t, uint64(5), bucketCumulative[1])
	assert.Equal(t, uint64(5), bucketCumulative[60])
}

// U-NFS-COLLECT-ZERO-EMITS-METRIC: an idle cluster (all-zero buckets) still
// emits a valid zero-count histogram without needing a fabricated
// Observe(0) sample.
func TestNFSPerformanceCollector_Collect_ZeroBucketsEmitsZeroCountHistogram(t *testing.T) {
	reg := prometheus.NewRegistry()
	stats := &NFSBasicStats{}
	stats.Optime.V3TimeBuckets = []float64{0, 0, 0, 0, 0, 0, 0, 0}
	mockClient := &mockNFSStatsClient{stats: stats}
	c := NewNFSPerformanceCollector(mockClient, reg, "test-cluster")

	require.NoError(t, c.Collect(context.Background()))
	hist := findHistogram(t, reg, "dell_powerscale_nfs_v3_latency_seconds", "test-cluster")
	require.NotNil(t, hist)
	assert.Equal(t, uint64(0), hist.GetSampleCount())
	assert.Equal(t, float64(0), hist.GetSampleSum())
}

// U-NFS-CLEANUP-REMOVES-LATENCY-SERIES: Cleanup removes the per-cluster
// latency snapshot so it no longer appears on subsequent scrapes.
func TestNFSPerformanceCollector_Cleanup_RemovesLatencySeries(t *testing.T) {
	reg := prometheus.NewRegistry()
	stats := &NFSBasicStats{}
	stats.Optime.V3TimeBuckets = []float64{5, 0, 0, 0, 0, 0, 0, 0}
	mockClient := &mockNFSStatsClient{stats: stats}
	c := NewNFSPerformanceCollector(mockClient, reg, "test-cluster")

	require.NoError(t, c.Collect(context.Background()))
	require.NotNil(t, findHistogram(t, reg, "dell_powerscale_nfs_v3_latency_seconds", "test-cluster"))

	c.Cleanup()
	assert.Nil(t, findHistogram(t, reg, "dell_powerscale_nfs_v3_latency_seconds", "test-cluster"))
}

// U-NFS-MULTI-CLUSTER-INDEPENDENT: two clusters sharing the same registry
// (and therefore the same underlying nfsLatencyCollector) must report
// independent snapshots.
func TestNFSPerformanceCollector_Collect_MultiClusterIndependent(t *testing.T) {
	reg := prometheus.NewRegistry()

	statsA := &NFSBasicStats{}
	statsA.Optime.V3TimeBuckets = []float64{10, 0, 0, 0, 0, 0, 0, 0}
	cA := NewNFSPerformanceCollector(&mockNFSStatsClient{stats: statsA}, reg, "cluster-a")

	statsB := &NFSBasicStats{}
	statsB.Optime.V3TimeBuckets = []float64{20, 0, 0, 0, 0, 0, 0, 0}
	cB := NewNFSPerformanceCollector(&mockNFSStatsClient{stats: statsB}, reg, "cluster-b")

	require.NoError(t, cA.Collect(context.Background()))
	require.NoError(t, cB.Collect(context.Background()))

	histA := findHistogram(t, reg, "dell_powerscale_nfs_v3_latency_seconds", "cluster-a")
	histB := findHistogram(t, reg, "dell_powerscale_nfs_v3_latency_seconds", "cluster-b")
	require.NotNil(t, histA)
	require.NotNil(t, histB)
	assert.Equal(t, uint64(10), histA.GetSampleCount())
	assert.Equal(t, uint64(20), histB.GetSampleCount())

	// Removing one cluster must not affect the other's series.
	cA.Cleanup()
	assert.Nil(t, findHistogram(t, reg, "dell_powerscale_nfs_v3_latency_seconds", "cluster-a"))
	histBAfter := findHistogram(t, reg, "dell_powerscale_nfs_v3_latency_seconds", "cluster-b")
	require.NotNil(t, histBAfter)
	assert.Equal(t, uint64(20), histBAfter.GetSampleCount())
}

// U-NFS-TRUNCATION-DOCUMENTED: buildLatencySnapshot only maps the first
// len(bucketBounds) raw OneFS values. This matches today's documented
// bucket-approximation behavior; it is intentionally unchanged pending the
// authoritative OneFS stats_nfs_optime_stats_v2 schema (see NOTE on
// buildLatencySnapshot). This test pins that documented, known-limited
// behavior so a future change to it is deliberate.
func TestBuildLatencySnapshot_TruncatesToBucketBoundsLength(t *testing.T) {
	bounds := []float64{0.1, 0.5, 1}
	raw := make([]float64, 242) // real OneFS v3_time_buckets length
	raw[0] = 7
	raw[1] = 3
	raw[2] = 1
	raw[8] = 999 // beyond len(bounds); must be ignored, not panic

	counts, sum := buildLatencySnapshot(raw, bounds)
	require.Len(t, counts, 3)
	assert.Equal(t, []uint64{7, 3, 1}, counts)
	assert.InDelta(t, 7*0.1+3*0.5+1*1, sum, 1e-9)
}

// U-NFS-LATENCY-COLLECTOR-DESCRIBE: Describe emits exactly one Desc.
func TestNFSLatencyCollector_Describe(t *testing.T) {
	c := newNFSLatencyCollector("test_hist", "help", []float64{1, 2}, []string{naming.LabelClusterName})
	ch := make(chan *prometheus.Desc, 2)
	c.Describe(ch)
	close(ch)
	var descs []*prometheus.Desc
	for d := range ch {
		descs = append(descs, d)
	}
	assert.Len(t, descs, 1)
	assert.True(t, strings.Contains(descs[0].String(), "test_hist"))
}

// U-NFS-LATENCY-COLLECTOR-NO-DATA: Collect emits nothing when no snapshot has been set.
func TestNFSLatencyCollector_Collect_NoData(t *testing.T) {
	c := newNFSLatencyCollector("test_hist", "help", []float64{1, 2}, []string{naming.LabelClusterName})
	ch := make(chan prometheus.Metric, 2)
	c.Collect(ch)
	close(ch)
	var count int
	for range ch {
		count++
	}
	assert.Equal(t, 0, count)
}

// U-NFS-LATENCY-COLLECTOR-DELETE-UNKNOWN: delete on an untracked cluster is a no-op.
func TestNFSLatencyCollector_Delete_UnknownClusterIsNoop(t *testing.T) {
	c := newNFSLatencyCollector("test_hist", "help", []float64{1, 2}, []string{naming.LabelClusterName})
	assert.NotPanics(t, func() {
		c.delete("never-set")
	})
}

func TestBuildLatencySnapshot_IgnoresNonFiniteCounts(t *testing.T) {
	counts, sum := buildLatencySnapshot(
		[]float64{math.NaN(), math.Inf(1), 2},
		[]float64{0.1, 0.5, 1},
	)

	assert.Equal(t, []uint64{0, 0, 2}, counts)
	assert.Equal(t, float64(2), sum)
}
