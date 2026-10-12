// Copyright © 2025-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collectors

import (
	"context"
	"fmt"
	"math"

	"github.com/dell/csm-metrics-common/pkg/naming"
	"github.com/prometheus/client_golang/prometheus"
)

// NFSBasicStats represents the basic NFS statistics from OneFS
type NFSBasicStats struct {
	Basic struct {
		SvcCounters struct {
			BytesRead    float64 `json:"bytes_read"`
			BytesWritten float64 `json:"bytes_written"`
		} `json:"svc_counters"`
	} `json:"basic"`
	Optime struct {
		V3TimeBuckets []float64 `json:"v3_time_buckets"`
		V4TimeBuckets []float64 `json:"v4_time_buckets"`
	} `json:"optime"`
}

// NFSStatsClient is the minimal interface for fetching NFS statistics
type NFSStatsClient interface {
	GetNFSStats(ctx context.Context) (*NFSBasicStats, error)
}

// NFSPerformanceCollector collects PowerScale NFS performance metrics
type NFSPerformanceCollector struct {
	client      NFSStatsClient
	clusterName string
	bytesRead   *prometheus.GaugeVec
	bytesWrite  *prometheus.GaugeVec
	latencyV3   *nfsLatencyCollector
	latencyV4   *nfsLatencyCollector
}

// NewNFSPerformanceCollector creates a new NFSPerformanceCollector and registers its metrics
func NewNFSPerformanceCollector(client NFSStatsClient, reg prometheus.Registerer, clusterName string) *NFSPerformanceCollector {
	c := &NFSPerformanceCollector{
		client:      client,
		clusterName: clusterName,
		bytesRead: registerOrGetGaugeVec(reg, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "dell_powerscale_nfs_read_throughput_bytes",
			Help: "PowerScale cluster NFS read throughput in bytes per second.",
		}, []string{naming.LabelClusterName})),
		bytesWrite: registerOrGetGaugeVec(reg, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "dell_powerscale_nfs_write_throughput_bytes",
			Help: "PowerScale cluster NFS write throughput in bytes per second.",
		}, []string{naming.LabelClusterName})),
		latencyV3: registerOrGetNFSLatencyCollector(reg, newNFSLatencyCollector(
			"dell_powerscale_nfs_v3_latency_seconds",
			"PowerScale cluster NFSv3 operation latency distribution in seconds.",
			naming.HistogramBuckets,
			[]string{naming.LabelClusterName},
		)),
		latencyV4: registerOrGetNFSLatencyCollector(reg, newNFSLatencyCollector(
			"dell_powerscale_nfs_v4_latency_seconds",
			"PowerScale cluster NFSv4 operation latency distribution in seconds.",
			naming.HistogramBuckets,
			[]string{naming.LabelClusterName},
		)),
	}
	return c
}

// Collect fetches NFS performance statistics and updates the Prometheus gauges.
//
// Latency buckets reported by OneFS (node.nfs.optime_stats) are converted
// into a per-cluster histogram snapshot rather than replayed through
// HistogramVec.Observe once per underlying operation count: replaying would
// make collection cost proportional to the (potentially very large,
// monotonically growing) OneFS operation counts, and would permanently
// inflate the in-process histogram on every collection cycle since
// Prometheus histograms never reset. buildLatencySnapshot keeps the cost of
// this step at O(len(naming.HistogramBuckets)) regardless of how many NFS
// operations OneFS has counted.
func (c *NFSPerformanceCollector) Collect(ctx context.Context) error {
	if c.client == nil {
		return fmt.Errorf("NFSPerformanceCollector: client is nil")
	}

	stats, err := c.client.GetNFSStats(ctx)
	if err != nil {
		return fmt.Errorf("NFSPerformanceCollector: failed to get NFS stats: %w", err)
	}

	if stats != nil {
		c.bytesRead.WithLabelValues(c.clusterName).Set(stats.Basic.SvcCounters.BytesRead)
		c.bytesWrite.WithLabelValues(c.clusterName).Set(stats.Basic.SvcCounters.BytesWritten)
	}

	if stats != nil {
		if len(stats.Optime.V3TimeBuckets) > 0 {
			bucketCounts, sum := buildLatencySnapshot(stats.Optime.V3TimeBuckets, naming.HistogramBuckets)
			c.latencyV3.set(c.clusterName, bucketCounts, sum)
		}
		if len(stats.Optime.V4TimeBuckets) > 0 {
			bucketCounts, sum := buildLatencySnapshot(stats.Optime.V4TimeBuckets, naming.HistogramBuckets)
			c.latencyV4.set(c.clusterName, bucketCounts, sum)
		}
	}

	return nil
}

// buildLatencySnapshot converts the raw OneFS optime values into exclusive
// counts aligned with the configured Prometheus bucket bounds and calculates
// an approximate sum for prometheus.NewConstHistogram. Each input value is
// treated as the count for the corresponding configured bucket and assigned
// the bucket's upper bound as its representative latency.
//
// Values beyond the configured bucket bounds are ignored so the snapshot
// remains compatible with the existing metric bucket layout. The resulting
// exclusive counts are converted to cumulative counts by nfsLatencyCollector
// when the metric is emitted.
func buildLatencySnapshot(rawBuckets, bucketBounds []float64) ([]uint64, float64) {
	bucketCounts := make([]uint64, len(bucketBounds))
	var sum float64
	for i, count := range rawBuckets {
		if i >= len(bucketBounds) {
			break
		}
		if count <= 0 || math.IsNaN(count) || math.IsInf(count, 0) {
			continue
		}
		n := uint64(count)
		bucketCounts[i] = n
		sum += count * bucketBounds[i]
	}
	return bucketCounts, sum
}

// Name returns the collector name
func (c *NFSPerformanceCollector) Name() string { return "NFSPerformanceCollector" }

// Cleanup removes all NFS performance metrics for this cluster
func (c *NFSPerformanceCollector) Cleanup() {
	labels := prometheus.Labels{naming.LabelClusterName: c.clusterName}
	c.bytesRead.DeletePartialMatch(labels)
	c.bytesWrite.DeletePartialMatch(labels)
	c.latencyV3.delete(c.clusterName)
	c.latencyV4.delete(c.clusterName)
}
